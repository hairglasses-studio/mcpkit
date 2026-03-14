package security

import (
	"context"
	"testing"
)

func TestParseRBACUsers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantNil  bool
		wantUser string
		wantRole Role
	}{
		{"empty", "", true, "", ""},
		{"whitespace", "  ", true, "", ""},
		{"single user", "alice:admin", false, "alice", RoleAdmin},
		{"operator", "bob:operator", false, "bob", RoleOperator},
		{"readonly", "viewer:readonly", false, "viewer", RoleReadonly},
		{"with spaces", " alice : admin , bob : operator ", false, "alice", RoleAdmin},
		{"invalid role skipped", "alice:superuser", true, "", ""},
		{"no colon skipped", "alice", true, "", ""},
		{"empty parts skipped", ":admin,bob:", true, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseRBACUsers(tt.input)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			roles, ok := result[tt.wantUser]
			if !ok {
				t.Fatalf("expected user %q in result", tt.wantUser)
			}
			found := false
			for _, r := range roles {
				if r == tt.wantRole {
					found = true
				}
			}
			if !found {
				t.Errorf("expected role %q for user %q, got %v", tt.wantRole, tt.wantUser, roles)
			}
		})
	}
}

func TestParseRBACUsers_MultipleUsers(t *testing.T) {
	result := ParseRBACUsers("alice:admin,bob:operator,default:readonly")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) != 3 {
		t.Errorf("expected 3 users, got %d", len(result))
	}
}

func TestRBAC_HasPermission(t *testing.T) {
	rbac := NewRBAC(RBACConfig{
		UserRoles: map[string][]Role{
			"admin_user": {RoleAdmin},
			"op_user":    {RoleOperator},
			"default":    {RoleReadonly},
		},
	})

	if !rbac.HasPermission("admin_user", PermAdmin) {
		t.Error("admin should have admin permission")
	}
	if !rbac.HasPermission("op_user", PermWrite) {
		t.Error("operator should have write permission")
	}
	if rbac.HasPermission("op_user", PermAdmin) {
		t.Error("operator should NOT have admin permission")
	}
	if rbac.HasPermission("unknown", PermWrite) {
		t.Error("readonly should NOT have write permission")
	}
}

func TestRBAC_CanAccessTool(t *testing.T) {
	rbac := NewRBAC(RBACConfig{
		UserRoles: map[string][]Role{
			"admin_user": {RoleAdmin},
			"op_user":    {RoleOperator},
		},
	})

	if !rbac.CanAccessTool("admin_user", "myapp_inventory_delete") {
		t.Error("admin should access delete tool")
	}
	if !rbac.CanAccessTool("op_user", "myapp_inventory_list") {
		t.Error("operator should access list tool")
	}
	if rbac.CanAccessTool("unknown", "myapp_inventory_delete") {
		t.Error("readonly should NOT access delete tool")
	}
}

func TestRBAC_SetUserRoles(t *testing.T) {
	rbac := NewRBAC(RBACConfig{})

	rbac.SetUserRoles("newuser", []Role{RoleOperator})
	roles := rbac.GetUserRoles("newuser")
	if len(roles) != 1 || roles[0] != RoleOperator {
		t.Errorf("expected [operator], got %v", roles)
	}
}

func TestRBAC_SetToolPermission(t *testing.T) {
	rbac := NewRBAC(RBACConfig{})

	perm := rbac.GetToolPermission("myapp_inventory_list")
	if perm != PermRead {
		t.Errorf("expected read for _list tool, got %v", perm)
	}

	rbac.SetToolPermission("myapp_inventory_list", PermAdmin)
	perm = rbac.GetToolPermission("myapp_inventory_list")
	if perm != PermAdmin {
		t.Errorf("expected admin after override, got %v", perm)
	}
}

func TestRBAC_GetToolPermission_DefaultsToRead(t *testing.T) {
	rbac := NewRBAC(RBACConfig{})

	perm := rbac.GetToolPermission("myapp_something_unknown")
	if perm != PermRead {
		t.Errorf("expected read for unknown suffix, got %v", perm)
	}
}

func TestRBAC_GetToolPermission_AllSuffixes(t *testing.T) {
	rbac := NewRBAC(RBACConfig{})

	readTools := []string{"my_status", "my_list", "my_get", "my_health", "my_whoami", "my_discover"}
	for _, tool := range readTools {
		if perm := rbac.GetToolPermission(tool); perm != PermRead {
			t.Errorf("tool %q: expected read, got %v", tool, perm)
		}
	}

	writeTools := []string{"my_sync", "my_create", "my_update", "my_add", "my_restart", "my_trigger"}
	for _, tool := range writeTools {
		if perm := rbac.GetToolPermission(tool); perm != PermWrite {
			t.Errorf("tool %q: expected write, got %v", tool, perm)
		}
	}

	adminTools := []string{"my_delete", "my_reset", "my_rotate"}
	for _, tool := range adminTools {
		if perm := rbac.GetToolPermission(tool); perm != PermAdmin {
			t.Errorf("tool %q: expected admin, got %v", tool, perm)
		}
	}
}

func TestRBAC_CheckAccess(t *testing.T) {
	rbac := NewRBAC(RBACConfig{
		UserRoles: map[string][]Role{
			"admin_user": {RoleAdmin},
		},
	})

	if err := rbac.CheckAccess(context.Background(), "admin_user", "my_delete"); err != nil {
		t.Errorf("admin should have access, got: %v", err)
	}
	if err := rbac.CheckAccess(context.Background(), "nobody", "my_delete"); err == nil {
		t.Error("readonly user should be denied access to delete tool")
	}
}

func TestGetAllRoles(t *testing.T) {
	roles := GetAllRoles()
	if len(roles) != 3 {
		t.Fatalf("expected 3 roles, got %d", len(roles))
	}

	roleNames := make(map[Role]bool)
	for _, r := range roles {
		roleNames[r.Name] = true
		if len(r.Permissions) == 0 {
			t.Errorf("role %q has no permissions", r.Name)
		}
	}

	for _, expected := range []Role{RoleAdmin, RoleOperator, RoleReadonly} {
		if !roleNames[expected] {
			t.Errorf("missing role %q in GetAllRoles()", expected)
		}
	}
}

func TestRBAC_GetUserAccess(t *testing.T) {
	rbac := NewRBAC(RBACConfig{
		UserRoles: map[string][]Role{
			"admin_user": {RoleAdmin},
		},
	})

	info := rbac.GetUserAccess("admin_user")
	if info.Username != "admin_user" {
		t.Errorf("username = %q, want admin_user", info.Username)
	}
	if len(info.Permissions) != 4 {
		t.Errorf("admin should have 4 permissions, got %d", len(info.Permissions))
	}

	info = rbac.GetUserAccess("nobody")
	if len(info.Permissions) != 1 {
		t.Errorf("readonly should have 1 permission, got %d", len(info.Permissions))
	}
}

func TestRBAC_DefaultRole(t *testing.T) {
	rbac := NewRBAC(RBACConfig{
		UserRoles:   map[string][]Role{"alice": {RoleAdmin}},
		DefaultRole: RoleOperator,
	})

	roles := rbac.GetUserRoles("unknown_user")
	if len(roles) != 1 || roles[0] != RoleOperator {
		t.Errorf("expected [operator] as default, got %v", roles)
	}
}
