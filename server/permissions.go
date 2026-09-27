package main

import (
	"fmt"
	"sort"
	"strings"
)

// Authorization is layered: permission -> role -> OIDC group. Every check in the
// server is written against a permission; roles bundle permissions and come from
// ART_ROLES; and ART_OIDC_GROUPS binds provider groups to roles. Nothing that
// enforces access needs to know about roles or groups - they are resolved once,
// at login, and only the resulting permission set travels on the request.
//
// Read access is deliberately not a permission: listing and downloading stay
// public unless ART_NO_LISTING turns the listing routes into authenticated ones.
//
// Permission strings are a compatibility surface. They travel inside issued
// session tokens (for at most ART_SESSION_TTL) and are decoded by the web UI for
// cosmetic gating, so new permissions are added, never renamed.
const (
	permFileCreate = "file:create"
	permFileDelete = "file:delete"
	permTagAdd     = "tag:add"
	permTagRemove  = "tag:remove"
)

// allPermissions lists every permission the server enforces, in the order logs
// and error messages present them. It is alphabetical, which is also the order a
// permission set renders in, so the two never disagree.
var allPermissions = []string{permFileCreate, permFileDelete, permTagAdd, permTagRemove}

// validPermission reports whether name is a permission this server enforces. It
// is what turns an unknown name in ART_ROLES into a startup error instead of a
// grant that silently never matches a check.
func validPermission(name string) bool {
	for _, known := range allPermissions {
		if name == known {
			return true
		}
	}
	return false
}

// permissionSet is the set of permissions a credential holds.
type permissionSet map[string]bool

// permissionSetFrom builds a set from the names carried by a credential.
func permissionSetFrom(names []string) permissionSet {
	set := make(permissionSet, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

// has reports whether the set contains perm. A nil set has nothing.
func (s permissionSet) has(perm string) bool { return s[perm] }

// names returns the permissions in a stable order, so a minted token and a log
// line do not depend on map iteration order.
func (s permissionSet) names() []string {
	names := make([]string, 0, len(s))
	for name := range s {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// listPermissions renders a permission list for a log line or an error message.
func listPermissions(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// parsePermissionList parses the comma-separated list the `token -perms` flag
// takes. An empty list is valid and grants nothing, so a minted token defaults
// to read-only rather than to whatever the static token can do.
func parsePermissionList(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	seen := permissionSet{}
	perms := make([]string, 0, len(allPermissions))
	for _, name := range strings.Split(raw, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if !validPermission(name) {
			return nil, fmt.Errorf("%q is not a permission this server knows (known permissions: %s)", name, strings.Join(allPermissions, ", "))
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		perms = append(perms, name)
	}

	sort.Strings(perms)
	return perms, nil
}
