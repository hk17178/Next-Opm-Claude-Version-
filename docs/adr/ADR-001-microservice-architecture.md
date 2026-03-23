# ADR-001: Microservice Architecture with Domain-Driven Design

- **Status**: Accepted
- **Date**: 2026-03-22
- **Decision Makers**: architect, team-lead
- **Tags**: architecture, microservices, DDD

## Context

OpsNexus is an enterprise-grade intelligent full-stack operations platform. The system must support seven distinct operational domains (Log, Alert, Incident, CMDB, Notification, AI, Analytics) that evolve independently, scale differently, and are owned by separate engineering teams. We need an architecture that supports independent deployment, fault isolation, and team autonomy while maintaining cross-domain consistency.

## Decision

Adopt a **microservice architecture** organized by domain-driven design (DDD) bounded contexts:

1. **7 Domain Services**: Each domain (log, alert, incident, cmdb, notify, ai, analytics) is an independent Go microservice with its own database schema and API surface.
2. **Shared Libraries**: Common concerns (auth, logging, tracing, error handling) are extracted into `pkg/shared/` as Go packages.
3. **API Gateway**: A single entry point handles routing, authentication, rate limiting, and request transformation.
4. **Monorepo Structure**: All services coexist in a single repository to simplify cross-cutting changes, CI/CD, and dependency management.
5. **Language Stack**: Go for all backend services; React (TypeScript) micro-frontend for the UI.

### Service Boundaries

| Service | Domain | Primary Responsibility |
|---------|--------|----------------------|
| svc-log | Log | Log ingestion, storage, search |
| svc-alert | Alert | Alert rule evaluation, firing, silencing |
| svc-incident | Incident | Incident lifecycle management |
| svc-cmdb | CMDB | Configuration item registry, topology |
| svc-notify | Notification | Multi-channel notification dispatch |
| svc-ai | AI | Intelligent analysis, root cause, prediction |
| svc-analytics | Analytics | Metrics aggregation, dashboards, reporting |

## Alternatives Considered

| Alternative | Pros | Cons |
|-------------|------|------|
| Monolithic application | Simpler deployment, no network overhead | Cannot scale domains independently; single point of failure; team coupling |
| SOA with ESB | Mature pattern, centralized governance | ESB becomes bottleneck; heavyweight; poor fit for cloud-native |
| Serverless (FaaS) | Auto-scaling, pay-per-use | Cold start latency; vendor lock-in; complex debugging; poor fit for long-running log processing |

## Consequences

### Positive

- Independent deployment and scaling per domain
- Fault isolation — failure in one service does not cascade
- Team autonomy — each domain team owns their service end-to-end
- Technology flexibility for future evolution (e.g., AI service could adopt Python sidecar)

### Negative

- Increased operational complexity (service discovery, distributed tracing, config management)
- Network latency for cross-service calls
- Data consistency requires eventual consistency patterns

### Risks

- Service boundary misalignment could lead to excessive cross-service calls — mitigated by DDD bounded context analysis
- Distributed debugging complexity — mitigated by mandatory OpenTelemetry instrumentation

## References

- Martin Fowler, "Microservices" (martinfowler.com)
- Sam Newman, "Building Microservices", 2nd Edition
- Eric Evans, "Domain-Driven Design"
