#!/usr/bin/env bash
# =============================================================================
# OpsNexus Database Migration Runner
#
# Runs golang-migrate against per-service PostgreSQL databases.
# Usage:
#   ./run-migrations.sh                     # migrate all 7 services
#   ./run-migrations.sh --service svc-log   # migrate one service
#   ./run-migrations.sh --dry-run           # show what would run
#   ./run-migrations.sh --dry-run --service svc-alert
# =============================================================================

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

SERVICES=(svc-log svc-alert svc-incident svc-cmdb svc-notify svc-ai svc-analytics)

# Default database connection (override via environment variables)
DB_HOST="${OPSNEXUS_DB_HOST:-localhost}"
DB_PORT="${OPSNEXUS_DB_PORT:-5432}"
DB_USER="${OPSNEXUS_DB_USER:-opsnexus}"
DB_PASSWORD="${OPSNEXUS_DB_PASSWORD:-changeme}"
DB_SSLMODE="${OPSNEXUS_DB_SSLMODE:-disable}"

# Per-service database name mapping
declare -A DB_NAMES=(
  [svc-log]="opm_log"
  [svc-alert]="opm_alert"
  [svc-incident]="opm_incident"
  [svc-cmdb]="opm_cmdb"
  [svc-notify]="opm_notify"
  [svc-ai]="opm_ai"
  [svc-analytics]="opm_analytics"
)

# Resolve project root (two levels up from this script)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------

DRY_RUN=false
TARGET_SERVICE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --service)
      TARGET_SERVICE="$2"
      shift 2
      ;;
    -h|--help)
      echo "Usage: $0 [--dry-run] [--service <svc-name>]"
      echo ""
      echo "Options:"
      echo "  --dry-run              Show migration commands without executing"
      echo "  --service <svc-name>   Run migration for a single service only"
      echo "                         Valid: ${SERVICES[*]}"
      echo "  -h, --help             Show this help"
      exit 0
      ;;
    *)
      echo "ERROR: Unknown argument: $1"
      echo "Run '$0 --help' for usage."
      exit 1
      ;;
  esac
done

# Validate target service if specified
if [[ -n "$TARGET_SERVICE" ]]; then
  valid=false
  for svc in "${SERVICES[@]}"; do
    if [[ "$svc" == "$TARGET_SERVICE" ]]; then
      valid=true
      break
    fi
  done
  if [[ "$valid" == "false" ]]; then
    echo "ERROR: Unknown service '$TARGET_SERVICE'"
    echo "Valid services: ${SERVICES[*]}"
    exit 1
  fi
  SERVICES=("$TARGET_SERVICE")
fi

# ---------------------------------------------------------------------------
# Pre-flight checks
# ---------------------------------------------------------------------------

if ! command -v migrate &>/dev/null; then
  echo "ERROR: 'migrate' CLI not found. Install golang-migrate:"
  echo "  https://github.com/golang-migrate/migrate"
  exit 1
fi

# ---------------------------------------------------------------------------
# Run migrations
# ---------------------------------------------------------------------------

FAILED=()
SUCCEEDED=()

for svc in "${SERVICES[@]}"; do
  db_name="${DB_NAMES[$svc]}"
  migrations_dir="${PROJECT_ROOT}/services/${svc}/migrations"

  if [[ ! -d "$migrations_dir" ]]; then
    echo "[SKIP] ${svc}: no migrations directory at ${migrations_dir}"
    continue
  fi

  dsn="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${db_name}?sslmode=${DB_SSLMODE}"

  echo "------------------------------------------------------"
  echo "[INFO] ${svc} -> ${db_name}"
  echo "       migrations: ${migrations_dir}"

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "[DRY-RUN] migrate -path ${migrations_dir} -database \"postgres://${DB_USER}:****@${DB_HOST}:${DB_PORT}/${db_name}?sslmode=${DB_SSLMODE}\" up"
    SUCCEEDED+=("$svc")
    continue
  fi

  if migrate -path "$migrations_dir" -database "$dsn" up; then
    echo "[OK] ${svc}: migration succeeded"
    SUCCEEDED+=("$svc")
  else
    echo "[FAIL] ${svc}: migration failed"
    FAILED+=("$svc")
  fi
done

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------

echo ""
echo "======================================================"
echo "Migration Summary"
echo "======================================================"
if [[ "$DRY_RUN" == "true" ]]; then
  echo "Mode: DRY-RUN (no changes applied)"
fi
echo "Succeeded: ${SUCCEEDED[*]:-none}"
if [[ ${#FAILED[@]} -gt 0 ]]; then
  echo "Failed:    ${FAILED[*]}"
  echo ""
  echo "ERROR: ${#FAILED[@]} service(s) failed migration. Review logs above."
  exit 1
fi
echo ""
echo "All migrations completed successfully."
