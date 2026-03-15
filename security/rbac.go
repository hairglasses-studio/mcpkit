package security

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Role represents a user role with associated permissions.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleReadonly Role = "readonly"
)

// Permission represents a specific action that can be performed.
type Permission string

const (
	PermRead   Permission = "read"
	PermWrite  Permission = "write"
	PermAdmin  Permission = "admin"
	PermDelete Permission = "delete"
)

// RolePermissions defines what permissions each role has.
var RolePermissions = map[Role][]Permission{
	RoleAdmin:    {PermRead, PermWrite, PermAdmin, PermDelete},
	RoleOperator: {PermRead, PermWrite},
	RoleReadonly: {PermRead},
}

// DefaultToolPermissions maps tool name suffixes to required permissions.
var DefaultToolPermissions = map[string]Permission{
	"_status":   PermRead,
	"_list":     PermRead,
	"_get":      PermRead,
	"_health":   PermRead,
	"_whoami":   PermRead,
	"_discover": PermRead,
	"_sync":     PermWrite,
	"_create":   PermWrite,
	"_update":   PermWrite,
	"_add":      PermWrite,
	"_restart":  PermWrite,
	"_trigger":  PermWrite,
	"_delete":   PermAdmin,
	"_reset":    PermAdmin,
	"_rotate":   PermAdmin,
}

// RBACConfig configures the RBAC system.
type RBACConfig struct {
	// UserRoles maps usernames to their roles.
	UserRoles map[string][]Role

	// DefaultRole is assigned to unknown users. Defaults to RoleReadonly.
	DefaultRole Role
}

// RBAC manages role-based access control.
type RBAC struct {
	mu            sync.RWMutex
	userRoles     map[string][]Role
	toolOverrides map[string]Permission
	defaultRole   Role
}

// NewRBAC creates a new RBAC instance with the given config.
func NewRBAC(config RBACConfig) *RBAC {
	userRoles := config.UserRoles
	if userRoles == nil {
		userRoles = make(map[string][]Role)
	}
	defaultRole := config.DefaultRole
	if defaultRole == "" {
		defaultRole = RoleReadonly
	}
	return &RBAC{
		userRoles:     userRoles,
		toolOverrides: make(map[string]Permission),
		defaultRole:   defaultRole,
	}
}

// ParseRBACUsers parses "user1:role1,user2:role2" into a role map.
// Returns nil if the input is empty or has no valid entries.
func ParseRBACUsers(env string) map[string][]Role {
	env = strings.TrimSpace(env)
	if env == "" {
		return nil
	}

	result := make(map[string][]Role)
	for _, entry := range strings.Split(env, ",") {
		entry = strings.TrimSpace(entry)
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			continue
		}
		username := strings.TrimSpace(parts[0])
		roleName := strings.TrimSpace(strings.ToLower(parts[1]))
		if username == "" || roleName == "" {
			continue
		}
		role := Role(roleName)
		if _, valid := RolePermissions[role]; !valid {
			continue
		}
		result[username] = append(result[username], role)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// GetUserRoles returns the roles for a user.
func (r *RBAC) GetUserRoles(username string) []Role {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if roles, ok := r.userRoles[username]; ok {
		return roles
	}
	if roles, ok := r.userRoles["default"]; ok {
		return roles
	}
	return []Role{r.defaultRole}
}

// HasPermission checks if a user has a specific permission.
func (r *RBAC) HasPermission(username string, perm Permission) bool {
	roles := r.GetUserRoles(username)
	for _, role := range roles {
		if perms, ok := RolePermissions[role]; ok {
			for _, p := range perms {
				if p == perm {
					return true
				}
			}
		}
	}
	return false
}

// CanAccessTool checks if a user can access a specific tool.
func (r *RBAC) CanAccessTool(username, toolName string) bool {
	requiredPerm := r.GetToolPermission(toolName)
	return r.HasPermission(username, requiredPerm)
}

// GetToolPermission returns the required permission for a tool.
func (r *RBAC) GetToolPermission(toolName string) Permission {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if perm, ok := r.toolOverrides[toolName]; ok {
		return perm
	}
	for suffix, perm := range DefaultToolPermissions {
		if len(toolName) >= len(suffix) && toolName[len(toolName)-len(suffix):] == suffix {
			return perm
		}
	}
	return PermRead
}

// SetUserRoles sets the roles for a user.
func (r *RBAC) SetUserRoles(username string, roles []Role) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.userRoles[username] = roles
}

// SetToolPermission sets a permission override for a tool.
func (r *RBAC) SetToolPermission(toolName string, perm Permission) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.toolOverrides[toolName] = perm
}

// CheckAccess checks if a user can access a tool, returning an error if denied.
func (r *RBAC) CheckAccess(_ context.Context, username, toolName string) error {
	if !r.CanAccessTool(username, toolName) {
		return fmt.Errorf("access denied: user %q lacks permission for tool %q", username, toolName)
	}
	return nil
}

// RoleInfo returns information about a role.
type RoleInfo struct {
	Name        Role         `json:"name"`
	Permissions []Permission `json:"permissions"`
}

// GetAllRoles returns information about all roles.
func GetAllRoles() []RoleInfo {
	var roles []RoleInfo
	for role, perms := range RolePermissions {
		roles = append(roles, RoleInfo{
			Name:        role,
			Permissions: perms,
		})
	}
	return roles
}

// UserAccessInfo returns access information for a user.
type UserAccessInfo struct {
	Username    string       `json:"username"`
	Roles       []Role       `json:"roles"`
	Permissions []Permission `json:"permissions"`
}

// GetUserAccess returns access information for a user.
func (r *RBAC) GetUserAccess(username string) UserAccessInfo {
	roles := r.GetUserRoles(username)
	permSet := make(map[Permission]bool)
	for _, role := range roles {
		if perms, ok := RolePermissions[role]; ok {
			for _, p := range perms {
				permSet[p] = true
			}
		}
	}
	var perms []Permission
	for p := range permSet {
		perms = append(perms, p)
	}
	return UserAccessInfo{
		Username:    username,
		Roles:       roles,
		Permissions: perms,
	}
}
