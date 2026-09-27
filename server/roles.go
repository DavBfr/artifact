package main

import (
	"encoding/json"
	"log"
	"os"
	"sort"
	"strings"
)

// Role names are configuration rather than code: ART_ROLES decides what exists,
// and ART_OIDC_GROUPS binds provider groups to those names. Only the built-in
// defaults below refer to the names directly.
const (
	roleCreator = "creator"
	roleTagger  = "tagger"
	roleDeleter = "deleter"
	roleAdmin   = "admin"

	// wildcardGroup in ART_OIDC_GROUPS binds roles to every authenticated user,
	// whoever the provider says they are (including someone with no groups).
	wildcardGroup = "*"

	// groupsClaimDefault is the id_token claim carrying group membership. Its
	// name varies by provider (Entra and Auth0 tend to namespace it), so it is
	// overridable with ART_OIDC_GROUPS_CLAIM.
	groupsClaimDefault = "groups"
)

// roleRegistry maps a role name to the permissions it grants.
type roleRegistry map[string][]string

// defaultRoles is the built-in registry, used unless ART_ROLES replaces it. It
// reproduces the split the flat group configuration was meant to express: a
// creator uploads and tags, a tagger moves tags, a deleter removes files, and
// holding all of them is an admin.
var defaultRoles = roleRegistry{
	roleCreator: {permFileCreate, permTagAdd},
	roleTagger:  {permTagRemove},
	roleDeleter: {permFileDelete},
	roleAdmin:   allPermissions,
}

var (
	// roles is the effective registry: the built-ins above, or whatever
	// ART_ROLES defined.
	roles roleRegistry

	// groupRoles binds an OIDC group to the roles its members are granted.
	groupRoles map[string][]string

	// groupsConfigured records whether ART_OIDC_GROUPS was set at all. When it
	// was not, groupRoles holds the built-in "every authenticated user is an
	// admin" binding, which is what keeps an upgrade from silently locking an
	// existing deployment out. It also decides whether the `groups` scope is
	// worth requesting from the provider.
	groupsConfigured bool
)

// initRolesConfig reads ART_ROLES and ART_OIDC_GROUPS. Both are admin-supplied
// JSON, so a malformed value or an unknown name is a fatal configuration error
// rather than something to paper over: a typo that silently grants - or silently
// withholds - access is worse than a server that refuses to start.
//
// The OIDC configuration is read afterwards, because whether group membership is
// worth requesting from the provider depends on groupsConfigured.
func initRolesConfig() {
	roles = parseRoles(os.Getenv("ART_ROLES"))

	raw := os.Getenv("ART_OIDC_GROUPS")
	groupsConfigured = raw != ""
	groupRoles = parseGroupRoles(raw)
}

// parseRoles returns the role registry described by ART_ROLES, or the built-in
// defaults when it is unset.
func parseRoles(raw string) roleRegistry {
	if raw == "" {
		return defaultRoles
	}

	parsed := roleRegistry{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		log.Fatalf("ART_ROLES must be a JSON object mapping each role to a list of permissions, e.g. {\"%s\":[\"%s\"]}: %v",
			roleCreator, permFileCreate, err)
	}
	if len(parsed) == 0 {
		log.Fatalf("ART_ROLES is an empty object: define at least one role, or unset it to use the built-in roles (%s)",
			strings.Join(roleNames(defaultRoles), ", "))
	}

	for role, perms := range parsed {
		if strings.TrimSpace(role) == "" {
			log.Fatalf("ART_ROLES contains an empty role name")
		}
		for _, perm := range perms {
			if !validPermission(perm) {
				log.Fatalf("ART_ROLES: role %q grants unknown permission %q (known permissions: %s)",
					role, perm, strings.Join(allPermissions, ", "))
			}
		}
		sort.Strings(parsed[role])
	}

	return parsed
}

// parseGroupRoles returns the group bindings described by ART_OIDC_GROUPS. An
// unset value means "every authenticated user is an admin", which is what the
// server did before roles existed: the login callback has always granted any
// identity the provider authenticated full rights.
func parseGroupRoles(raw string) map[string][]string {
	builtIn := raw == ""

	bindings := builtInGroupRoles()
	if !builtIn {
		bindings = map[string][]string{}
		if err := json.Unmarshal([]byte(raw), &bindings); err != nil {
			log.Fatalf("ART_OIDC_GROUPS must be a JSON object mapping each group to a list of roles, e.g. {\"admins\":[\"%s\"]}: %v",
				roleAdmin, err)
		}
	}

	for group, roleList := range bindings {
		if strings.TrimSpace(group) == "" {
			log.Fatalf("ART_OIDC_GROUPS contains an empty group name")
		}
		for _, role := range roleList {
			if _, ok := roles[role]; ok {
				continue
			}
			if builtIn {
				// The built-in binding names roleAdmin, so replacing the
				// registry without an admin role leaves it dangling. Say so
				// plainly instead of reporting an unknown role, since the
				// operator never wrote the binding that failed.
				log.Fatalf("ART_OIDC_GROUPS is unset, so every authenticated user would be granted the built-in role %q, but ART_ROLES does not define it: keep an %q role or set ART_OIDC_GROUPS explicitly",
					role, roleAdmin)
			}
			log.Fatalf("ART_OIDC_GROUPS: group %q maps unknown role %q (known roles: %s)",
				group, role, strings.Join(roleNames(roles), ", "))
		}
	}

	return bindings
}

// builtInGroupRoles is the binding used when ART_OIDC_GROUPS is unset.
func builtInGroupRoles() map[string][]string {
	return map[string][]string{wildcardGroup: {roleAdmin}}
}

// resolvePermissions maps an id_token's groups to the permissions their roles
// grant. It runs once, at login, so the result can be baked into the session
// token and verifying a request never has to consult this configuration.
func resolvePermissions(groups []string) []string {
	if len(groupRoles) == 0 {
		return nil
	}

	perms := permissionSet{}

	// The wildcard applies to every authenticated user, including one the
	// provider reports no groups for at all.
	addRoles(perms, groupRoles[wildcardGroup])
	for _, group := range groups {
		addRoles(perms, groupRoles[group])
	}

	return perms.names()
}

// addRoles unions each role's permissions into the set. Unknown roles cannot
// reach here - parseGroupRoles refuses to start on one.
func addRoles(into permissionSet, roleList []string) {
	for _, role := range roleList {
		for _, perm := range roles[role] {
			into[perm] = true
		}
	}
}

// logAuthzConfig reports the effective policy once at startup, so an operator
// can see what a deployment actually grants without reading two JSON blobs back
// out of the environment.
func logAuthzConfig() {
	log.Printf("Authorization roles: %s", describeRoles())

	if !oidcEnabled() {
		log.Printf("Authorization: OIDC is not configured, so ART_OIDC_GROUPS does not apply; only ART_API_TOKEN (all permissions) and minted tokens are accepted")
		return
	}

	switch {
	case !groupsConfigured:
		log.Printf("Authorization: ART_OIDC_GROUPS is unset, so every authenticated user is granted %s (set it to restrict access)",
			listPermissions(groupRoles[wildcardGroup]))
	case len(groupRoles) == 0:
		log.Printf("Authorization: ART_OIDC_GROUPS is empty, so authenticated users can read but not change anything")
	default:
		for _, group := range sortedGroups(groupRoles) {
			log.Printf("Authorization: OIDC group %s -> %s", group, listPermissions(groupRoles[group]))
		}
	}
}

// describeRoles renders the registry as "name(perm, perm)" entries.
func describeRoles() string {
	names := roleNames(roles)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"("+listPermissions(roles[name])+")")
	}
	return strings.Join(parts, " ")
}

// roleNames returns the registry's role names in a stable order.
func roleNames(registry roleRegistry) []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// sortedGroups returns the bound group names in a stable order.
func sortedGroups(bindings map[string][]string) []string {
	groups := make([]string, 0, len(bindings))
	for group := range bindings {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}
