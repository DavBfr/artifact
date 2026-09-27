package main

import (
	"reflect"
	"testing"
)

// samePermissions compares two permission lists, treating "no permissions" as
// the same thing whether it is a nil slice or an empty one: the claim is omitted
// either way.
func samePermissions(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// withRolesConfig installs a role configuration for one test and restores the
// previous one afterwards. An empty string means the variable was unset, which
// is a different thing from an empty JSON object and has to stay distinguishable.
func withRolesConfig(t *testing.T, rolesJSON, groupsJSON string) {
	t.Helper()

	previousRoles, previousGroupRoles, previousConfigured := roles, groupRoles, groupsConfigured
	t.Cleanup(func() {
		roles, groupRoles, groupsConfigured = previousRoles, previousGroupRoles, previousConfigured
	})

	// parseGroupRoles validates role names against the registry, so the registry
	// has to be in place first.
	roles = parseRoles(rolesJSON)
	groupsConfigured = groupsJSON != ""
	groupRoles = parseGroupRoles(groupsJSON)
}

// The built-in registry is what a deployment that configures nothing gets.
func TestDefaultRolesMatchTheDocumentedDefaults(t *testing.T) {
	withRolesConfig(t, "", "")

	want := roleRegistry{
		roleCreator: {permFileCreate, permTagAdd},
		roleTagger:  {permTagRemove},
		roleDeleter: {permFileDelete},
		roleAdmin:   {permFileCreate, permFileDelete, permTagAdd, permTagRemove},
	}
	if !reflect.DeepEqual(roles, want) {
		t.Errorf("default roles = %v, want %v", roles, want)
	}
}

// Unset ART_OIDC_GROUPS means every authenticated user is an admin, which is what
// an OIDC login granted before roles existed. Changing this would silently lock
// existing deployments out on upgrade.
func TestUnsetGroupBindingsGrantEveryoneEverything(t *testing.T) {
	withRolesConfig(t, "", "")

	if groupsConfigured {
		t.Error("groupsConfigured = true with ART_OIDC_GROUPS unset")
	}

	for name, groups := range map[string][]string{
		"no groups":   nil,
		"one group":   {"anything"},
		"many groups": {"a", "b"},
	} {
		if got := resolvePermissions(groups); !samePermissions(got, allPermissions) {
			t.Errorf("%s: resolvePermissions = %v, want %v", name, got, allPermissions)
		}
	}
}

// An explicit binding grants a role to the named groups and nobody else: the
// wildcard is what says "everybody", so its absence is meaningful.
func TestExplicitGroupBindingsGrantOnlyListedGroups(t *testing.T) {
	withRolesConfig(t, "", `{"admins":["admin"],"dev":["creator"]}`)

	tests := []struct {
		name   string
		groups []string
		want   []string
	}{
		{name: "dev", groups: []string{"dev"}, want: []string{permFileCreate, permTagAdd}},
		{name: "admins", groups: []string{"admins"}, want: allPermissions},
		{name: "unlisted group", groups: []string{"contractors"}, want: nil},
		{name: "no groups", groups: nil, want: nil},
		// Membership unions, and the result is sorted and deduplicated.
		{name: "both", groups: []string{"dev", "admins"}, want: allPermissions},
		{name: "duplicate groups", groups: []string{"dev", "dev"}, want: []string{permFileCreate, permTagAdd}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := resolvePermissions(test.groups)
			if !samePermissions(got, test.want) {
				t.Errorf("resolvePermissions(%v) = %v, want %v", test.groups, got, test.want)
			}
		})
	}
}

// "*" is the explicit "all authenticated users", including one the provider
// reports no groups for, and it unions with any named group.
func TestWildcardGroupAppliesToEveryone(t *testing.T) {
	withRolesConfig(t, "", `{"*":["creator"],"admins":["admin"]}`)

	if got := resolvePermissions(nil); !samePermissions(got, []string{permFileCreate, permTagAdd}) {
		t.Errorf("no groups: resolvePermissions = %v, want the wildcard role's permissions", got)
	}
	if got := resolvePermissions([]string{"admins"}); !samePermissions(got, allPermissions) {
		t.Errorf("admins: resolvePermissions = %v, want %v", got, allPermissions)
	}
}

// An empty binding is the deliberate lock-down: authenticated users can read.
func TestEmptyGroupBindingsGrantNothing(t *testing.T) {
	withRolesConfig(t, "", `{}`)

	if !groupsConfigured {
		t.Error("groupsConfigured = false for an explicitly empty ART_OIDC_GROUPS")
	}
	if got := resolvePermissions([]string{"admins"}); len(got) != 0 {
		t.Errorf("resolvePermissions = %v, want nothing", got)
	}
}

// A role whose permission list is empty keeps the name valid - so a group
// binding that mentions it still starts - while granting nothing.
func TestEmptyRoleKeepsTheNameValid(t *testing.T) {
	withRolesConfig(t, `{"creator":[],"admin":["file:create"]}`, `{"*":["creator"],"admins":["admin"]}`)

	if got := resolvePermissions(nil); len(got) != 0 {
		t.Errorf("wildcard over an empty role = %v, want nothing", got)
	}
	if got := resolvePermissions([]string{"admins"}); !samePermissions(got, []string{permFileCreate}) {
		t.Errorf("admins = %v, want [%s]", got, permFileCreate)
	}
}

// ART_ROLES replaces the built-in registry rather than merging into it, so a
// deployment that defines one role has exactly one role.
func TestArtRolesReplacesTheBuiltIns(t *testing.T) {
	withRolesConfig(t, `{"uploader":["file:create","tag:add"]}`, `{"*":["uploader"]}`)

	if _, ok := roles[roleCreator]; ok {
		t.Error("the built-in creator role survived ART_ROLES")
	}
	if _, ok := roles[roleAdmin]; ok {
		t.Error("the built-in admin role survived ART_ROLES")
	}

	want := []string{permFileCreate, permTagAdd}
	if got := resolvePermissions(nil); !samePermissions(got, want) {
		t.Errorf("resolvePermissions = %v, want %v", got, want)
	}
}

func TestParsePermissionList(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   []string
		wantOK bool
	}{
		{name: "empty grants nothing", input: "", want: nil, wantOK: true},
		{name: "whitespace grants nothing", input: "   ", want: nil, wantOK: true},
		{name: "single", input: "file:create", want: []string{permFileCreate}, wantOK: true},
		{name: "several", input: "tag:remove,file:create", want: []string{permFileCreate, permTagRemove}, wantOK: true},
		{name: "trimmed and deduplicated", input: " file:create , file:create ", want: []string{permFileCreate}, wantOK: true},
		{name: "trailing comma", input: "file:create,", want: []string{permFileCreate}, wantOK: true},
		{name: "unknown", input: "file:create,file:destroy", wantOK: false},
		{name: "role name is not a permission", input: "admin", wantOK: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parsePermissionList(test.input)
			if !test.wantOK {
				if err == nil {
					t.Fatalf("parsePermissionList(%q) accepted an unknown permission: %v", test.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePermissionList(%q): %v", test.input, err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("parsePermissionList(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

// The static token is the one credential that always carries every permission:
// CI pipelines and the curl examples depend on it doing exactly what it did
// before roles existed.
func TestStaticTokenCarriesEveryPermission(t *testing.T) {
	withAuthConfig(t, "static-token", "")

	auth, ok := resolveAuth(bearerRequest("static-token"))
	if !ok {
		t.Fatal("the static token was rejected")
	}
	if auth.Kind != authKindStatic {
		t.Errorf("Kind = %q, want %q", auth.Kind, authKindStatic)
	}
	if got := auth.Perms.names(); !samePermissions(got, allPermissions) {
		t.Errorf("static token permissions = %v, want %v", got, allPermissions)
	}
}

// A session token carries exactly the permissions it was minted with, so an
// upgrade that predates the claim yields a read-only credential rather than an
// accidental grant.
func TestSessionTokenCarriesOnlyItsOwnPermissions(t *testing.T) {
	withAuthConfig(t, "", testSecret)

	auth, ok := resolveAuth(bearerRequest(mintSessionTokenWithPerms(t, permFileCreate)))
	if !ok {
		t.Fatal("the session token was rejected")
	}
	if auth.Kind != authKindSession {
		t.Errorf("Kind = %q, want %q", auth.Kind, authKindSession)
	}
	if auth.Subject != "tester" {
		t.Errorf("Subject = %q, want %q", auth.Subject, "tester")
	}
	if got, want := auth.Perms.names(), []string{permFileCreate}; !samePermissions(got, want) {
		t.Errorf("permissions = %v, want %v", got, want)
	}

	// No claim at all is the pre-roles token shape.
	auth, ok = resolveAuth(bearerRequest(mintSessionTokenWithPerms(t)))
	if !ok {
		t.Fatal("a token with no perms claim was rejected")
	}
	if len(auth.Perms) != 0 {
		t.Errorf("a token with no perms claim resolved to %v, want nothing", auth.Perms.names())
	}
}
