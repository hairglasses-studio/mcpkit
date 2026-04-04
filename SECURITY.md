# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in mcpkit, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please email **security@hairglasses-studio.dev** with:

1. A description of the vulnerability
2. Steps to reproduce (if applicable)
3. The potential impact
4. Any suggested fixes

## Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial assessment**: Within 1 week
- **Fix or mitigation**: Depends on severity, typically within 2 weeks for critical issues

## Scope

This policy applies to the mcpkit Go module and all packages within it. Security issues in dependencies should be reported to the respective upstream projects.

## Supported Versions

| Version | Supported |
|---------|-----------|
| v0.1.x  | Yes       |

## Security Features

mcpkit includes several security-oriented packages:

- **`sanitize`** — Input/output sanitization, secret/PII redaction, injection filtering
- **`auth`** — JWT/JWKS validation, OAuth 2.1, DPoP proof validation
- **`security`** — RBAC, audit logging, tenant isolation
- **`secrets`** — Secret provider interface with env/file backends
