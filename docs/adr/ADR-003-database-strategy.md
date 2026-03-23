# ADR-003: Polyglot Persistence — Database-per-Service Strategy

- **Status**: Accepted
- **Date**: 2026-03-22
- **Decision Makers**: architect, team-lead
- **Tags**: architecture, database, persistence

## Context

Each domain service has distinct data access patterns, consistency requirements, and scalability needs. Log storage requires high write throughput and full-text search. CMDB needs relational integrity for topology graphs. Alert rules require fast key-value lookups. A single database technology cannot optimally serve all domains.

## Decision

Adopt a **database-per-service** strategy with technology selection based on each domain's access patterns:

| Service | Primary Store | Rationale |
|---------|--------------|-----------|
| svc-log | Elasticsearch (OpenSearch) | Full-text search, log aggregation, high write throughput |
| svc-alert | PostgreSQL + Redis | Relational rule storage; Redis for real-time state and deduplication windows |
| svc-incident | PostgreSQL | Relational data with complex lifecycle state machines |
| svc-cmdb | PostgreSQL | Relational integrity for CI relationships and topology graphs |
| svc-notify | PostgreSQL + Redis | Delivery tracking; Redis for rate limiting and dedup |
| svc-ai | PostgreSQL + Vector Store (pgvector) | Model metadata; vector similarity for knowledge retrieval |
| svc-analytics | ClickHouse | Columnar storage for OLAP queries, time-series aggregation |

### Key Principles

1. **No shared databases**: Each service owns its schema exclusively. Cross-service data access is via API or events only.
2. **Schema migrations**: Managed via `golang-migrate` with migration files in each service's `migrations/` directory.
3. **Connection pooling**: PgBouncer for PostgreSQL services in production.
4. **Backup strategy**: Defined per data tier — hot (real-time replication), warm (hourly snapshots), cold (daily to object storage).

## Alternatives Considered

| Alternative | Pros | Cons |
|-------------|------|------|
| Single shared PostgreSQL | Simple operations, ACID across services | Tight coupling; schema conflicts; cannot optimize per-domain; single point of failure |
| All-in-one MongoDB | Flexible schema, easy horizontal scaling | Poor fit for relational CMDB data; no native full-text search at ES scale; weak ACID |
| TimescaleDB for everything | Good time-series support on PostgreSQL | Not optimal for full-text search (logs) or OLAP (analytics) |

## Consequences

### Positive

- Each service uses the best-fit storage technology for its workload
- Independent scaling — log storage can grow without affecting CMDB
- No cross-service schema coupling
- Freedom to evolve storage technology per domain

### Negative

- Operational complexity — multiple database technologies to maintain
- No cross-service joins — must denormalize or use API composition
- Increased infrastructure cost from running multiple database systems

### Risks

- Data inconsistency across services — mitigated by Saga pattern and compensating transactions for critical flows
- Operational burden — mitigated by managed cloud database services and Infrastructure-as-Code

## References

- Chris Richardson, "Microservices Patterns" — Database per Service pattern
- Martin Fowler, "Polyglot Persistence"
