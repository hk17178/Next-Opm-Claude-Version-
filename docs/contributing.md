# OpsNexus Contributing Guide

## Table of Contents

- [Shared Package Usage](#shared-package-usage)
- [Go Backend Conventions](#go-backend-conventions)
- [React Frontend Conventions](#react-frontend-conventions)
- [Internationalization (i18n)](#internationalization-i18n)
- [Kafka Event Conventions](#kafka-event-conventions)
- [gRPC Inter-Service Communication](#grpc-inter-service-communication)
- [API Conventions](#api-conventions)
- [Git Workflow](#git-workflow)
- [MR Review Standards](#mr-review-standards)

---

## Shared Package Usage

All shared Go packages live under `pkg/` and MUST be used by every service. Do not duplicate functionality that exists in shared packages.

### Package Index

| Package | Purpose | When to Use |
|---------|---------|-------------|
| `pkg/event` | Kafka CloudEvents producer/consumer | All async inter-service communication |
| `pkg/auth` | JWT validation + Keycloak integration | Every HTTP handler behind auth |
| `pkg/logger` | Structured logging (zap) with trace context | All logging — never use `fmt.Println` |
| `pkg/errors` | Unified error types and domain error codes | All error creation and handling |
| `pkg/httputil` | HTTP response helpers and pagination | All HTTP handler response writing |
| `pkg/health` | Health check endpoints (liveness/readiness) | Every service must register `/healthz` and `/readyz` |
| `pkg/middleware` | HTTP middleware (logging, recovery, rate limit, tracing, CORS, request ID) | Applied at router level |
| `pkg/proto` | gRPC service definitions | Synchronous inter-service calls |

### Usage Rules

1. **MUST use `pkg/logger`** — Initialize with `logger.New("svc-{domain}")`. Never use `fmt.Println`, `log.Println`, or create your own logger.

2. **MUST use `pkg/errors`** — Return `*errors.AppError` from service layer. Use domain-specific error codes (e.g., `errors.ErrAlertRuleNotFound`). Add new domain codes to `pkg/errors/errors.go`.

3. **MUST use `pkg/httputil`** — Write all HTTP responses via `httputil.JSON()`, `httputil.Error()`, or `httputil.PagedJSON()`. Never write raw `json.NewEncoder(w).Encode()` in handlers.

4. **MUST use `pkg/event`** for Kafka — Create events via `event.NewCloudEvent()`. Always set `PartitionKey` for ordering. Use predefined topic/type constants from `event.go`.

5. **MUST use `pkg/health`** — Every service must register health checks:
   ```go
   h := health.New("svc-log", version)
   h.AddReadinessCheck("postgres", health.DatabaseCheck(dbPool))
   h.AddReadinessCheck("kafka", health.PingCheck(kafkaBroker, 3*time.Second))
   ```

6. **MUST apply standard middleware** — Every service HTTP router must include:
   ```go
   r.Use(middleware.Recovery(log))
   r.Use(middleware.RequestID)
   r.Use(middleware.Tracing)
   r.Use(middleware.Logging(log))
   r.Use(auth.Middleware(keyStore, log))
   ```

7. **MUST use `pkg/auth`** for authorization — Use `auth.RequireRole()` or `auth.RequirePermission()` middleware on protected endpoints. Extract user with `auth.FromContext(ctx)`.

8. **Adding new shared code** — Any code used by 2+ services MUST go into `pkg/`. Get architect approval before adding new shared packages.

### Example: Service Initialization

```go
package main

import (
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/opsnexus/opsnexus/pkg/auth"
    "github.com/opsnexus/opsnexus/pkg/health"
    "github.com/opsnexus/opsnexus/pkg/logger"
    "github.com/opsnexus/opsnexus/pkg/middleware"
)

func main() {
    log := logger.New("svc-alert")
    defer log.Sync()

    // Health checks
    h := health.New("svc-alert", "0.1.0")
    h.AddReadinessCheck("postgres", health.DatabaseCheck(db))

    // Router with standard middleware stack
    r := chi.NewRouter()
    r.Use(middleware.Recovery(log))
    r.Use(middleware.RequestID)
    r.Use(middleware.Tracing)
    r.Use(middleware.Logging(log))
    r.Use(middleware.CORS(middleware.DefaultCORSConfig()))

    // Health endpoints (unauthenticated)
    r.Get("/healthz", h.LivenessHandler())
    r.Get("/readyz", h.ReadinessHandler())

    // Authenticated routes
    r.Group(func(r chi.Router) {
        r.Use(auth.Middleware(keyStore, log))
        r.Route("/api/v1/alert", func(r chi.Router) {
            // ... domain handlers
        })
    })

    http.ListenAndServe(":8080", r)
}
```

---

## Go Backend Conventions

### Project Structure (per service)

```
services/svc-{domain}/
  cmd/                  # Entry points
    server/
      main.go
  internal/             # Private application code
    handler/            # HTTP/gRPC handlers
    service/            # Business logic
    repository/         # Data access layer
    model/              # Domain models
    dto/                # Data transfer objects
  migrations/           # Database migrations (golang-migrate)
  api/                  # OpenAPI spec (symlink or copy)
  configs/              # Service-specific config
  Dockerfile
  Makefile
```

### Code Style

- **Formatter**: `gofmt` / `goimports` (enforced by CI)
- **Linter**: `golangci-lint` with the project `.golangci.yml` config
- **Naming**: Follow [Effective Go](https://go.dev/doc/effective_go) naming conventions
  - Exported types: `PascalCase`
  - Unexported: `camelCase`
  - Acronyms: `HTTPClient`, not `HttpClient`
- **Error handling**: Always check errors. Use `fmt.Errorf("context: %w", err)` for wrapping.
- **Context**: All public functions that do I/O must accept `context.Context` as the first parameter.
- **Logging**: Use the shared structured logger (`pkg/shared/logger`). No `fmt.Println` or `log.Println`.
- **Testing**: Table-driven tests preferred. Test files in the same package as the code under test.

### Dependencies

- HTTP framework: `net/http` with `chi` router
- Database: `pgx/v5` for PostgreSQL
- Migrations: `golang-migrate`
- Config: `viper`
- Logging: `slog` (stdlib structured logging)
- Tracing: `OpenTelemetry Go SDK`
- Testing: `testing` + `testify`

### Error Response Format

All API error responses must follow the standard format:

```json
{
  "code": "VALIDATION_ERROR",
  "message": "Human-readable error message",
  "details": {}
}
```

---

## React Frontend Conventions

### Project Structure

```
web/
  packages/
    shell/              # Host application (micro-frontend shell)
    shared/             # Shared components, hooks, utilities
    mf-{domain}/        # Micro-frontend per domain
      src/
        components/     # React components
        hooks/          # Custom hooks
        pages/          # Page-level components
        services/       # API client functions
        stores/         # State management (Zustand)
        i18n/           # Translation files
        types/          # TypeScript types
```

### Code Style

- **Language**: TypeScript (strict mode). No `any` unless absolutely necessary with a comment explaining why.
- **Formatter**: Prettier (enforced by CI)
- **Linter**: ESLint with project config
- **Components**: Functional components only. No class components.
- **Naming**:
  - Components: `PascalCase` (e.g., `AlertRuleList.tsx`)
  - Hooks: `camelCase` with `use` prefix (e.g., `useAlertRules.ts`)
  - Utilities: `camelCase` (e.g., `formatTimestamp.ts`)
  - Types/Interfaces: `PascalCase` with descriptive names (e.g., `AlertRule`, not `IAlertRule`)
- **State management**: Zustand for client state. React Query (TanStack Query) for server state.
- **Styling**: Tailwind CSS + component library (Ant Design or Shadcn/ui).

### Testing

- Unit tests: Vitest + React Testing Library
- Component tests: Focus on behavior, not implementation
- E2E tests: Playwright (critical paths only)

---

## Internationalization (i18n)

### MANDATORY REQUIREMENT

All user-facing strings MUST be internationalized. This is a hard requirement for every MR.

### Rules

1. **No hardcoded strings** in UI components. Every user-visible string must go through the i18n system.
2. **Supported languages**: `zh-CN` (Chinese Simplified) as primary, `en-US` (English) as secondary.
3. **Translation library**: `react-i18next`
4. **Key naming**: Hierarchical dot notation, scoped by domain and feature.
   ```
   alert.rule.create.title = "Create Alert Rule"
   alert.rule.create.submit = "Submit"
   incident.list.empty = "No incidents found"
   ```
5. **Translation files**: JSON format in each micro-frontend's `i18n/` directory.
   ```
   i18n/
     zh-CN.json
     en-US.json
   ```
6. **Backend error messages**: Backend returns error codes. Frontend maps codes to i18n keys.
7. **Date/time formatting**: Use `dayjs` with locale support. Never format dates manually.
8. **Number formatting**: Use `Intl.NumberFormat` for locale-aware number display.

### MR Checklist for i18n

- [ ] All new user-facing strings use i18n keys
- [ ] Both `zh-CN` and `en-US` translation files are updated
- [ ] No hardcoded Chinese or English strings in components
- [ ] Date/time displays use locale-aware formatting

---

## Kafka Event Conventions

### Event Envelope

All events use CloudEvents v1.0 specification:

```json
{
  "specversion": "1.0",
  "id": "uuid-v4",
  "type": "opsnexus.{domain}.{event_type}",
  "source": "/services/svc-{domain}",
  "time": "RFC 3339 timestamp",
  "datacontenttype": "application/json",
  "data": { ... }
}
```

### Topic Naming

Format: `opsnexus.{domain}.{event_type}`

### Schema Validation

- All event schemas are defined in `schemas/events/` as JSON Schema files
- Schema validation is enforced in CI for producers
- Breaking changes require a new schema version (e.g., `opsnexus.alert.fired.v2`)

### Consumer Group Naming

Format: `{consuming-service}.{topic-short-name}`

Example: `svc-alert.log-ingested`

---

## API Conventions

### General Rules

- All APIs follow OpenAPI 3.0 specification
- Base path: `/api/v1/{domain}`
- Authentication: JWT Bearer token
- Pagination: Cursor-based (`page_token` + `page_size`)
- Sorting: `sort` parameter with `asc`/`desc`
- Filtering: Query parameters for simple filters, request body for complex queries
- Response envelope: Direct resource or list, no wrapper object

### HTTP Methods

| Method | Usage |
|--------|-------|
| GET | Read resources (idempotent) |
| POST | Create resources or execute actions |
| PUT | Full resource replacement |
| PATCH | Partial resource update |
| DELETE | Remove resources |

### Status Codes

| Code | Usage |
|------|-------|
| 200 | Success with body |
| 201 | Resource created |
| 202 | Accepted for async processing |
| 204 | Success with no body |
| 400 | Client error / validation failure |
| 401 | Unauthenticated |
| 403 | Forbidden |
| 404 | Resource not found |
| 409 | Conflict (duplicate) |
| 429 | Rate limited |
| 500 | Internal server error |

### Versioning

- URL path versioning: `/api/v1/`, `/api/v2/`
- Breaking changes require a new version
- Non-breaking additions (new optional fields) do not require version bump

---

## gRPC Inter-Service Communication

### When to Use gRPC vs Kafka

| Pattern | Use Case | Example |
|---------|----------|---------|
| **Kafka (async)** | Fire-and-forget events, fan-out, event sourcing | Alert fired -> Incident + Notify + AI |
| **gRPC (sync)** | Request-response, data enrichment, queries | AI fetches logs for analysis context |

**Rule of thumb**: If the caller needs the response to continue, use gRPC. If the caller doesn't need to wait, use Kafka.

### Proto Definitions

All `.proto` files live in `pkg/proto/{domain}/`. Service definitions:

| Proto | Service | Used By |
|-------|---------|---------|
| `pkg/proto/log/log_service.proto` | LogService | svc-ai, svc-alert |
| `pkg/proto/alert/alert_service.proto` | AlertService | svc-incident, svc-ai |
| `pkg/proto/incident/incident_service.proto` | IncidentService | svc-ai, svc-notify |
| `pkg/proto/cmdb/cmdb_service.proto` | CMDBService | svc-alert, svc-ai, svc-notify |
| `pkg/proto/notify/notify_service.proto` | NotifyService | svc-incident, svc-ai |
| `pkg/proto/ai/ai_service.proto` | AIService | svc-incident, svc-analytics |
| `pkg/proto/analytics/analytics_service.proto` | AnalyticsService | svc-ai, dashboard |

### Proto Code Generation

```bash
# Install buf CLI
go install github.com/bufbuild/buf/cmd/buf@latest

# Generate Go code from protos
cd pkg/proto && buf generate
```

Generated code goes to `pkg/proto/gen/go/` and is checked into the repository.

### Proto Conventions

1. **Common types** in `pkg/proto/common/common.proto` — reuse `PageRequest`, `PageResponse`, `TimeRange`, `KeyValue`.
2. **Package naming**: `opsnexus.{domain}` (e.g., `opsnexus.alert`).
3. **Go package**: `github.com/opsnexus/opsnexus/pkg/proto/{domain}`.
4. **Breaking changes** detected by `buf breaking` in CI.
5. **Enum zero values** must be `UNSPECIFIED` (e.g., `OPERATION_STATUS_UNSPECIFIED = 0`).

---

## Git Workflow

### Branch Naming

```
feature/{domain}-{short-description}    # New features
fix/{domain}-{short-description}         # Bug fixes
refactor/{domain}-{short-description}    # Refactoring
docs/{short-description}                 # Documentation
chore/{short-description}                # Tooling, CI, deps
```

### Commit Messages

Follow Conventional Commits:

```
{type}({scope}): {description}

[optional body]

[optional footer]
```

Types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `ci`, `perf`

Scope: Service or domain name (e.g., `svc-log`, `svc-alert`, `web`, `infra`)

Examples:
```
feat(svc-alert): add silence management endpoints
fix(svc-log): handle malformed timestamp in log ingestion
docs(api): update svc-cmdb OpenAPI spec with topology query
```

---

## MR Review Standards

### Automated Checks (must pass)

- [ ] CI pipeline green (lint, test, build)
- [ ] No decrease in test coverage
- [ ] OpenAPI spec validation passes
- [ ] Kafka schema validation passes
- [ ] Docker image builds successfully

### Reviewer Checklist

#### Architecture Compliance
- [ ] Changes respect service boundaries (no cross-service database access)
- [ ] New API endpoints follow OpenAPI conventions
- [ ] New events follow CloudEvents + JSON Schema conventions
- [ ] No circular dependencies between packages

#### Code Quality
- [ ] Functions are focused and reasonably sized
- [ ] Error handling is comprehensive (no swallowed errors)
- [ ] Context propagation is correct
- [ ] No TODO/FIXME without a linked issue
- [ ] Logging is structured and at appropriate levels

#### Security
- [ ] No secrets or credentials in code
- [ ] Input validation at API boundaries
- [ ] SQL queries use parameterized statements
- [ ] Auth/RBAC checks on new endpoints

#### i18n (Frontend MRs)
- [ ] All user-facing strings are internationalized
- [ ] Both zh-CN and en-US translations provided
- [ ] No hardcoded display strings in components

#### Testing
- [ ] New code has corresponding tests
- [ ] Tests cover both happy path and error cases
- [ ] Integration tests for new API endpoints
- [ ] No test pollution (tests clean up after themselves)

#### Documentation
- [ ] OpenAPI spec updated for API changes
- [ ] Event schema updated for Kafka changes
- [ ] ADR created for significant architectural decisions

### Approval Requirements

- **Domain-internal changes**: 1 approval from domain engineer or architect
- **Cross-domain changes**: Approval from architect + affected domain engineers
- **Infrastructure changes**: Approval from infrastructure engineer + architect
- **Schema/API contract changes**: Approval from architect (mandatory)
