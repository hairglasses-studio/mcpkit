// Package security provides RBAC, audit logging, audit export, and tenant
// context propagation for MCP servers.
//
// Role-based access control is configured via [RBACConfig] (mapping users to
// [Role] values) and applied as a [registry.Middleware] by [RBACMiddleware].
// All tool invocations are recorded by [AuditLogger] as structured [AuditEvent]
// values; events can be streamed to JSONL files or arbitrary writers using the
// [AuditExporter] interface ([JSONLExporter], [StreamExporter]). Multi-tenant
// deployments attach a tenant identifier to the context via
// [WithTenantID]/[TenantIDFromContext].
//
// Example — attach RBAC and audit middleware:
//
//	rbac := security.NewRBAC(security.RBACConfig{
//	    UserRoles: map[string][]security.Role{"alice": {security.RoleAdmin}},
//	})
//	logger, _ := security.NewAuditLogger(security.AuditLoggerConfig{LogFile: "audit.jsonl"})
//	reg := registry.New(registry.Config{
//	    Middleware: []registry.Middleware{
//	        security.RBACMiddleware(rbac),
//	        security.AuditMiddleware(logger),
//	    },
//	})
package security
