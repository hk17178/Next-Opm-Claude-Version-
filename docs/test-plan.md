# OpsNexus Test Plan

## 1. Test Coverage Baseline

### Current Test File Inventory

| Service | Layer | Test File | Test Count | Coverage Focus |
|---------|-------|-----------|------------|----------------|
| svc-alert | biz | engine_test.go | 8 tests | Layer 0-4 pipeline, dedup, disabled rules, fingerprint, severity |
| svc-alert | biz | dedup_test.go | 7 tests | Fingerprint stability, dedup window, expiry |
| svc-alert | biz | baseline_test.go | 13 tests | Welford algorithm, anomaly detection, peak exemption |
| svc-alert | contract | consumer_test.go | 5 tests | CloudEvent schema, ironclad flag, forward compat |
| svc-log | biz | ingest_test.go | 18 tests | HTTP/Kafka ingest, masking, sensitive fields, CloudEvent |
| svc-log | biz | search_test.go | 16 tests | Search pagination, time range, filters, stats, export |
| svc-incident | biz | incident_usecase_test.go | 16 tests | CRUD, status transitions, postmortem, metrics, assignment |
| svc-incident | biz | model_test.go | 15 tests | State machine, postmortem requirements, MTTA/MTTI/MTTR |
| svc-incident | service | handler_test.go | ~10 tests | HTTP handler integration |
| svc-cmdb | biz | asset_usecase_test.go | 15 tests | Asset CRUD, relations, topology, groups, dimensions, discovery |
| svc-cmdb | biz | model_test.go | exists | Model validation |
| svc-cmdb | service | handler_test.go | ~10 tests | HTTP handler integration |
| svc-ai | biz | desensitizer_test.go | 7 tests | Password/IP/email redaction, blocked fields, hash |
| svc-ai | biz | circuit_breaker_test.go | 5 tests | State transitions, threshold, blocking |
| svc-ai | biz | context_collector_test.go | exists | Context collection |
| svc-ai | biz | model_test.go | exists | Model definitions |
| svc-ai | contract | consumer_test.go | exists | CloudEvent contract |
| svc-notify | biz | broadcaster_test.go | 7 tests | Severity filter, severity mapping, truncation, message build |
| svc-notify | biz | dedup_test.go | 3 tests | Dedup key determinism, uniqueness, length |
| svc-notify | biz | model_test.go | exists | Model definitions |
| svc-notify | contract | consumer_test.go | exists | CloudEvent contract |
| svc-analytics | biz | sla_test.go | 12 tests | SLA calculation, error budget, dimensions, validation |
| svc-analytics | biz | metrics_test.go | 14 tests | Ingest routing, query, correlation, Pearson, anomaly score |
| svc-analytics | biz | budget_alert_test.go | 4 tests | Budget alert bands (healthy/warning/critical) |
| svc-analytics | contract | consumer_test.go | exists | CloudEvent contract |

### Coverage Gaps Identified

| Service | Missing Coverage | Priority |
|---------|-----------------|----------|
| svc-alert | RuleUseCase (CRUD, validation) - no tests for rule.go | HIGH |
| svc-alert | FrequencyCounter - no unit tests for frequency_counter.go | HIGH |
| svc-alert | Layer 1 frequency rule in engine (evalFrequency) | MEDIUM |
| svc-alert | Layer 3 trend detection in engine (evalTrend) | MEDIUM |
| svc-alert | data layer (alert_repo, rule_repo) | LOW (requires DB) |
| svc-ai | CircuitBreaker half-open recovery flow | MEDIUM |
| svc-ai | Desensitizer SanitizeMap nested recursion edge cases | LOW |
| svc-notify | Broadcaster full integration (Broadcast method) | MEDIUM |
| svc-notify | ChannelManager/ChannelSender - no tests | HIGH |
| svc-notify | HealthProbe - no tests | MEDIUM |
| svc-log | data layer (log_repo, ES repo) | LOW (requires infra) |
| svc-analytics | Dashboard/Report/Knowledge biz logic | MEDIUM |
| All services | service/handler layer - partial coverage | MEDIUM |
| All services | Kafka consumer integration | LOW (requires Kafka) |

## 2. Unit Test Targets

### Target: >= 70% coverage per service biz layer

| Service | Estimated Current Coverage | Target | Gap |
|---------|---------------------------|--------|-----|
| svc-alert/biz | ~65% (rule.go, frequency_counter.go untested) | 80% | +15% |
| svc-log/biz | ~80% (well covered) | 80% | OK |
| svc-incident/biz | ~85% (comprehensive) | 80% | OK |
| svc-cmdb/biz | ~75% (good coverage) | 80% | +5% |
| svc-ai/biz | ~60% (circuit_breaker half-open, desensitizer edge) | 75% | +15% |
| svc-notify/biz | ~50% (broadcaster integration, channel_sender) | 70% | +20% |
| svc-analytics/biz | ~70% (dashboard, report, knowledge untested) | 75% | +5% |

## 3. Priority Test Additions (Phase 1 - Immediate)

### 3.1 svc-alert: FrequencyCounter Tests
- Record and Count within window
- Count outside window returns 0
- Multiple keys independent
- Cleanup removes old entries
- Concurrent access safety

### 3.2 svc-alert: RuleUseCase Tests
- CreateRule with valid input
- CreateRule validation (empty name, nil condition, invalid layer)
- UpdateRule existing/not found
- DeleteRule
- ListRules pagination and pageSize cap

### 3.3 svc-alert: Engine Frequency + Trend Tests
- Frequency rule triggers after N events in window
- Frequency rule does not trigger below threshold
- Trend rule detects upward change
- Trend rule detects downward change
- Trend "either" direction

### 3.4 svc-ai: CircuitBreaker Half-Open Recovery
- Open -> HalfOpen after timeout
- HalfOpen -> Closed after success threshold
- HalfOpen -> Open on failure
- IsOpen returns false when timeout expired

### 3.5 svc-notify: Channel Sender / Health Probe
- DedupEngine basic operations
- Health probe status reporting

## 4. Integration Test Scenarios

### 4.1 Alert Pipeline (requires Kafka + PostgreSQL)
1. Metric sample -> Kafka -> svc-alert consumer -> engine evaluation -> alert persisted
2. Log event -> Kafka -> svc-alert consumer -> keyword match -> alert fired
3. Alert fired -> CloudEvent published -> svc-notify receives -> notification sent
4. Ironclad alert bypasses dedup -> multiple alerts created

### 4.2 Log Pipeline (requires Kafka + Elasticsearch)
1. HTTP ingest -> validate -> mask PII -> buffer -> bulk index to ES
2. Kafka ingest -> parse JSON -> mask -> index
3. Search with Lucene query + time range + filters -> paginated results
4. Stats aggregation by level -> correct bucket counts

### 4.3 Incident Lifecycle (requires PostgreSQL)
1. Create incident -> auto-assign via oncall schedule -> timeline entry
2. Status flow: created -> triaging -> assigned -> resolving -> resolved -> closed
3. P0 incident requires postmortem before close
4. Escalation changes severity + creates timeline entry
5. MTTA/MTTI/MTTR calculation with real timestamps

### 4.4 CMDB Operations (requires PostgreSQL)
1. Asset CRUD with auto-generated ID
2. Relation creation validates both assets exist
3. Topology query returns graph with correct depth
4. Discovery approval creates or links asset
5. Maintenance cascade propagation

### 4.5 Analytics Pipeline (requires ClickHouse + PostgreSQL)
1. Metric ingest routes business vs resource metrics correctly
2. SLA calculation: (total - downtime) / total * 100%
3. Error budget alert fires at warning (< 20%) and critical (< 5%)
4. Pearson correlation between CPU and latency metrics
5. Anomaly detection z-score > 3 sigma

### 4.6 Notification Pipeline (requires Redis + external webhooks)
1. Alert fired event -> broadcaster -> severity filter -> dedup check -> send
2. Incident created -> template rendering -> WeChat Work webhook
3. Duplicate notification suppressed within window
4. Disabled bot skipped

## 5. E2E Test Scenarios

### 5.1 Critical Business Flow
1. **Full Alert-to-Resolution**: Log ingested -> alert fired -> incident created -> assigned -> resolved -> SLA calculated
2. **AI Analysis Flow**: Incident created -> AI analysis triggered -> desensitized input -> result returned -> notification sent
3. **CMDB-Aware Alert**: Alert with host_id -> CMDB lookup -> cascade impact analysis -> incident with affected_assets

### 5.2 Edge Cases
1. Ironclad alert during notification service downtime -> retry
2. Circuit breaker opens for AI model -> fallback behavior
3. Concurrent identical alerts -> only one passes dedup
4. SLA error budget exhausted -> critical alert fired

## 6. Performance Test Baselines

| Scenario | Target | Metric |
|----------|--------|--------|
| Log ingest throughput | >= 10,000 entries/sec | Entries indexed per second |
| Alert engine evaluation | < 5ms per rule | p99 latency per rule evaluation |
| Search query response | < 500ms for 10K results | p95 response time |
| SLA calculation | < 200ms per config | Single config calculation time |
| Notification send | < 1s per message | End-to-end send latency |

## 7. Test Environment Requirements

### Infrastructure Dependencies
- PostgreSQL 15+ (svc-alert, svc-incident, svc-cmdb, svc-ai, svc-notify, svc-analytics)
- Elasticsearch 8.x (svc-log)
- ClickHouse (svc-analytics)
- Redis 7+ (svc-notify dedup)
- Kafka (all services for CloudEvent pub/sub)

### Test Configuration
- Unit tests: No external dependencies (mock repositories)
- Integration tests: Docker Compose with test-specific databases
- E2E tests: Full infrastructure via `deploy/docker-compose`

## 8. Test Execution Commands

```bash
# All unit tests
make test

# Single service with coverage
make test-svc-alert
make test-svc-log
make test-svc-incident
make test-svc-cmdb
make test-svc-ai
make test-svc-notify
make test-svc-analytics

# Coverage report
make test-coverage

# Frontend tests
cd frontend && pnpm test
cd frontend && pnpm run type-check
cd frontend && pnpm run lint
```
