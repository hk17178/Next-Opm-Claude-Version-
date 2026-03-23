# ADR-005: Unified Observability with OpenTelemetry

- **Status**: Accepted
- **Date**: 2026-03-22
- **Decision Makers**: architect, team-lead
- **Tags**: architecture, observability, tracing, metrics, logging

## Context

With 7 microservices communicating via Kafka and REST APIs, debugging production issues requires end-to-end visibility. We need distributed tracing, structured logging, and metrics collection that work consistently across all services. The observability stack must correlate logs, traces, and metrics for a single request flow.

## Decision

1. **OpenTelemetry (OTel)** as the unified instrumentation framework for all Go services.
2. **Three pillars** integrated through trace context propagation:
   - **Traces**: Distributed tracing with W3C TraceContext propagation
   - **Metrics**: Runtime and business metrics exported via OTel
   - **Logs**: Structured logging with `slog`, enriched with trace/span IDs

### Instrumentation Stack

| Signal | Library | Collector | Backend |
|--------|---------|-----------|---------|
| Traces | OTel Go SDK | OTel Collector | Jaeger / Tempo |
| Metrics | OTel Go SDK | OTel Collector | Prometheus / VictoriaMetrics |
| Logs | slog + OTel bridge | OTel Collector | Elasticsearch (via svc-log) |

### Mandatory Instrumentation

Every service MUST instrument:

1. **HTTP handlers**: Request duration, status code, method, path
2. **Kafka producers/consumers**: Message publish/consume latency, partition, topic
3. **Database queries**: Query duration, operation type, table
4. **External HTTP calls**: Outbound request duration, status, target service
5. **Business events**: Domain-specific counters (alerts fired, incidents created, etc.)

### Trace Context Propagation

- HTTP: `traceparent` / `tracestate` headers (W3C standard)
- Kafka: Trace context in CloudEvents `traceparent` extension attribute
- All log entries include `trace_id` and `span_id` fields

### Standard Metric Names

Format: `opsnexus_{service}_{metric}_{unit}`

Examples:
- `opsnexus_svc_log_ingest_total` (counter)
- `opsnexus_svc_alert_evaluation_duration_seconds` (histogram)
- `opsnexus_svc_incident_open_count` (gauge)

### Shared Middleware

`pkg/shared/middleware/` provides:
- `otelhttp` handler wrapper for automatic HTTP instrumentation
- Kafka consumer/producer interceptors with trace propagation
- Database query tracer for pgx
- Structured log middleware that injects trace context

## Alternatives Considered

| Alternative | Pros | Cons |
|-------------|------|------|
| Vendor-specific SDKs (Datadog, New Relic) | Rich features out of the box | Vendor lock-in; cost; inconsistent across services |
| Manual instrumentation | Full control | Inconsistent, error-prone, high maintenance burden |
| Zipkin for tracing | Mature, simple | Narrower ecosystem than OTel; no unified metrics/logs story |

## Consequences

### Positive

- Single instrumentation standard across all services
- Vendor-agnostic — can switch backends without code changes
- Correlated logs/traces/metrics for rapid incident investigation
- OTel Collector provides buffering, batching, and multi-backend export

### Negative

- OTel SDK adds some overhead (mitigated by sampling)
- Initial setup complexity for OTel Collector pipeline configuration
- Team must learn OTel API/SDK concepts

### Risks

- High-cardinality metrics (per-user, per-request-ID) can cause storage explosion — mitigated by metric naming guidelines and cardinality limits
- Trace sampling decisions must be consistent across services — mitigated by head-based sampling at gateway

## References

- OpenTelemetry Specification
- W3C Trace Context
- Google SRE Book — Chapter on Monitoring Distributed Systems
