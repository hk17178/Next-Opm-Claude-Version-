# ADR-004: JWT-Based Authentication with RBAC Authorization

- **Status**: Accepted
- **Date**: 2026-03-22
- **Decision Makers**: architect, team-lead
- **Tags**: architecture, security, auth, RBAC

## Context

OpsNexus requires a unified authentication and authorization mechanism across all 7 domain services. The platform serves multiple user roles (admin, operator, viewer, API integrator) and must support both interactive UI sessions and programmatic API access. Each service must independently verify identity and permissions without coupling to a central auth service for every request.

## Decision

1. **Authentication**: JWT (JSON Web Token) Bearer tokens issued by a dedicated auth service (or integrated with enterprise SSO/OIDC provider).
2. **Token Format**: Signed JWT with claims including `sub` (user ID), `roles`, `permissions`, `org_id`, `exp`.
3. **Token Verification**: Each service verifies JWT signature locally using a shared public key (asymmetric RS256). No per-request callback to auth service.
4. **Authorization**: Role-Based Access Control (RBAC) with predefined roles and permissions.
5. **API Gateway**: Validates JWT, injects user context headers (`X-User-Id`, `X-User-Roles`) for downstream services.
6. **Service-to-Service Auth**: Internal mTLS for service mesh communication. Services trust gateway-injected headers for user context.

### RBAC Model

| Role | Description | Permissions |
|------|-------------|-------------|
| admin | Full platform access | All operations |
| operator | Manage alerts, incidents, CMDB | Create/update/delete on alert, incident, cmdb; read on all |
| analyst | View and analyze | Read on all; create analysis tasks; manage dashboards |
| viewer | Read-only access | Read on all resources |
| api_integrator | Programmatic access | Scoped per API key configuration |

### Token Claims

```json
{
  "sub": "user-uuid",
  "org_id": "org-uuid",
  "roles": ["operator"],
  "permissions": ["alert:write", "incident:write", "cmdb:write", "log:read"],
  "iss": "opsnexus-auth",
  "exp": 1711180800,
  "iat": 1711094400
}
```

### Permission Format

`{domain}:{action}` where action is `read`, `write`, `delete`, `admin`.

## Alternatives Considered

| Alternative | Pros | Cons |
|-------------|------|------|
| Session-based auth | Simple implementation | Doesn't scale across services; sticky sessions needed |
| OAuth2 opaque tokens | Standard, revocable | Requires token introspection on every request; central dependency |
| API keys only | Simple for integrations | No user identity; hard to implement fine-grained RBAC |

## Consequences

### Positive

- Stateless auth — services verify tokens independently, no central bottleneck
- Standard JWT ecosystem with broad library support
- Fine-grained RBAC with domain-scoped permissions
- Supports both UI sessions and API integrations

### Negative

- Token revocation requires short-lived tokens + refresh token rotation (or a token blacklist)
- JWT payload size grows with permissions — mitigated by using role-based claims and server-side permission resolution

### Risks

- Key rotation must be coordinated across all services — mitigated by JWKS endpoint with key rotation support
- Over-permissive roles — mitigated by principle of least privilege in default role definitions

## References

- RFC 7519 (JWT)
- OpenID Connect Core 1.0
- NIST RBAC Model
