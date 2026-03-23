# ADR-002: Kafka as Event Bus with CloudEvents Envelope

- **Status**: Accepted
- **Date**: 2026-03-22
- **Decision Makers**: architect, team-lead
- **Tags**: architecture, messaging, kafka, events

## Context

The seven domain services need an asynchronous communication channel for event-driven workflows. Examples include: log ingestion triggering alert evaluation, alert firing creating incidents, incidents dispatching notifications, and AI analysis consuming events from multiple domains. The system must handle high throughput (log ingestion), guarantee at-least-once delivery, and support event replay for debugging and analytics.

## Decision

1. **Apache Kafka** as the event bus for all inter-service asynchronous communication.
2. **CloudEvents v1.0 envelope** for all event payloads to ensure interoperability and self-describing messages.
3. **Topic naming convention**: `opsnexus.{domain}.{event-type}` (e.g., `opsnexus.log.ingested`, `opsnexus.alert.fired`).
4. **JSON Schema** for event payload validation, stored in `schemas/events/`.
5. **Schema versioning**: Append major version to type field when breaking changes occur (e.g., `opsnexus.alert.fired.v2`). Backward-compatible changes do not require version bump.
6. **Partition strategy**: Partition by resource ID (e.g., host_id, alert_id) to guarantee ordering within a resource.

### Core Event Topics

| Topic | Producer | Primary Consumers |
|-------|----------|-------------------|
| opsnexus.log.ingested | svc-log | svc-alert, svc-ai, svc-analytics |
| opsnexus.alert.fired | svc-alert | svc-incident, svc-notify, svc-ai |
| opsnexus.incident.created | svc-incident | svc-notify, svc-ai, svc-analytics |
| opsnexus.ai.analysis.done | svc-ai | svc-incident, svc-notify, svc-analytics |
| opsnexus.notify.sent | svc-notify | svc-analytics |

### Consumer Group Naming

Format: `{consuming-service}.{topic-short-name}` (e.g., `svc-alert.log-ingested`).

## Alternatives Considered

| Alternative | Pros | Cons |
|-------------|------|------|
| RabbitMQ | Simpler setup, flexible routing | Lower throughput; no built-in replay; not ideal for log-scale data |
| NATS JetStream | Lightweight, low latency | Smaller ecosystem; less mature persistence; fewer operational tools |
| Redis Streams | Already in stack for caching | Not designed for durable high-throughput eventing; limited consumer group features |
| gRPC streaming | Low latency, type-safe | Point-to-point only; no fan-out; no replay; tight coupling |

## Consequences

### Positive

- High throughput for log ingestion workloads (millions of events/sec)
- Durable event replay for debugging, analytics backfill, and new consumer onboarding
- Decoupled producers and consumers — adding new consumers requires no producer changes
- CloudEvents envelope provides self-describing, tooling-compatible messages

### Negative

- Operational overhead of running Kafka cluster (ZooKeeper or KRaft)
- Eventual consistency between services
- Message ordering only guaranteed within a partition

### Risks

- Schema evolution mismanagement could break consumers — mitigated by JSON Schema validation in CI and schema registry
- Consumer lag could cause delayed alert/notification processing — mitigated by monitoring consumer group lag with alerts

## References

- CloudEvents Specification v1.0 (cloudevents.io)
- Apache Kafka Documentation
- Confluent Schema Registry
