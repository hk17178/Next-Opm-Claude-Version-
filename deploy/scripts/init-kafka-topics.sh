#!/usr/bin/env bash
# =============================================================================
# OpsNexus Kafka Topic Initialization Script
#
# Waits for Kafka to become ready, then creates all business topics idempotently
# (existing topics are skipped).
#
# Usage:
#   ./init-kafka-topics.sh                          # use defaults
#   KAFKA_BOOTSTRAP=kafka:9092 ./init-kafka-topics.sh
# =============================================================================

set -euo pipefail

KAFKA_BOOTSTRAP="${KAFKA_BOOTSTRAP:-localhost:9092}"
KAFKA_BIN="${KAFKA_BIN:-/opt/bitnami/kafka/bin}"
REPLICATION_FACTOR="${KAFKA_REPLICATION_FACTOR:-1}"
MAX_RETRIES="${KAFKA_READY_RETRIES:-30}"
RETRY_INTERVAL="${KAFKA_READY_INTERVAL:-5}"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

wait_kafka() {
  echo "Waiting for Kafka at ${KAFKA_BOOTSTRAP}..."
  local attempt=1
  while [[ $attempt -le $MAX_RETRIES ]]; do
    if "${KAFKA_BIN}/kafka-broker-api-versions.sh" \
        --bootstrap-server "${KAFKA_BOOTSTRAP}" >/dev/null 2>&1; then
      echo "Kafka is ready (attempt ${attempt}/${MAX_RETRIES})."
      return 0
    fi
    echo "  attempt ${attempt}/${MAX_RETRIES} - not ready, retrying in ${RETRY_INTERVAL}s..."
    sleep "${RETRY_INTERVAL}"
    attempt=$((attempt + 1))
  done
  echo "ERROR: Kafka did not become ready after ${MAX_RETRIES} attempts."
  exit 1
}

create_topic() {
  local topic="$1"
  local partitions="$2"
  local retention_ms="$3"
  local extra_configs="${4:-}"

  # Check if topic already exists (idempotent)
  if "${KAFKA_BIN}/kafka-topics.sh" \
      --bootstrap-server "${KAFKA_BOOTSTRAP}" \
      --describe --topic "${topic}" >/dev/null 2>&1; then
    echo "[SKIP] ${topic} already exists"
    return 0
  fi

  local config_args="--config retention.ms=${retention_ms}"
  if [[ -n "${extra_configs}" ]]; then
    config_args="${config_args} --config ${extra_configs}"
  fi

  echo "[CREATE] ${topic} (partitions=${partitions}, retention=${retention_ms}ms)"
  "${KAFKA_BIN}/kafka-topics.sh" \
    --bootstrap-server "${KAFKA_BOOTSTRAP}" \
    --create \
    --topic "${topic}" \
    --partitions "${partitions}" \
    --replication-factor "${REPLICATION_FACTOR}" \
    ${config_args}
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

echo "======================================================"
echo "OpsNexus Kafka Topic Initialization"
echo "======================================================"
echo "Bootstrap: ${KAFKA_BOOTSTRAP}"
echo "Replication factor: ${REPLICATION_FACTOR}"
echo ""

wait_kafka

# Retention constants
RETENTION_1D=$((  1 * 24 * 60 * 60 * 1000))   #  86400000
RETENTION_7D=$((  7 * 24 * 60 * 60 * 1000))   # 604800000
RETENTION_30D=$(( 30 * 24 * 60 * 60 * 1000))  # 2592000000
RETENTION_90D=$(( 90 * 24 * 60 * 60 * 1000))  # 7776000000

echo ""
echo "--- Creating business topics ---"

# Log ingestion pipeline (high-throughput)
create_topic "opsnexus.log.ingested"      6  "${RETENTION_7D}"  "max.message.bytes=1073741824"

# Alert lifecycle
create_topic "opsnexus.alert.fired"       6  "${RETENTION_30D}"

# Incident lifecycle
create_topic "opsnexus.incident.created"  3  "${RETENTION_90D}"

# AI analysis results
create_topic "opsnexus.ai.analysis.done"  3  "${RETENTION_30D}"

# Notification delivery tracking
create_topic "opsnexus.notify.sent"       3  "${RETENTION_7D}"

# Internal: raw log ingestion (high-throughput, short retention)
create_topic "opm.log.ingest"             12 "${RETENTION_1D}"

echo ""
echo "--- Verifying topics ---"
"${KAFKA_BIN}/kafka-topics.sh" \
  --bootstrap-server "${KAFKA_BOOTSTRAP}" \
  --list | grep -E "^(opsnexus\.|opm\.)" | sort

echo ""
echo "======================================================"
echo "Kafka topic initialization complete."
echo "======================================================"
