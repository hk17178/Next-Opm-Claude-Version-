#!/usr/bin/env bash
# =============================================================================
# OpsNexus 一键启动脚本
# 用法：./deploy/start-all.sh
# 支持幂等运行（重复执行安全）
# =============================================================================
set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
log_info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_step()  { echo -e "\n${CYAN}==== $* ====${NC}"; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DC_DIR="${SCRIPT_DIR}/docker-compose"

# -------------------------------------------------------------------------
# 前置检查
# -------------------------------------------------------------------------
log_step "前置检查"

if ! command -v docker &>/dev/null; then
  log_error "Docker 未安装。请先执行：sudo ./deploy/scripts/prepare-host.sh"
  exit 1
fi

if ! docker compose version &>/dev/null; then
  log_error "Docker Compose 插件未安装。请重新安装 Docker（包含 docker-compose-plugin）"
  exit 1
fi

if [[ ! -f "${DC_DIR}/.env" ]]; then
  log_error ".env 文件不存在！"
  log_info "请执行：cp deploy/docker-compose/.env.example deploy/docker-compose/.env"
  log_info "然后编辑 .env 文件修改密码，再重新运行此脚本。"
  exit 1
fi

# 检查 .env 中是否还有 changeme 密码
if grep -q "^POSTGRES_PASSWORD=changeme" "${DC_DIR}/.env"; then
  log_warn "检测到 POSTGRES_PASSWORD 仍为默认值 'changeme'，建议修改！"
fi

log_info "前置检查通过"

# -------------------------------------------------------------------------
# Step 1: 启动数据库
# -------------------------------------------------------------------------
log_step "[1/7] 启动 PostgreSQL 数据库（7个实例）"
cd "${DC_DIR}"
docker compose up -d \
  pg-log pg-alert pg-incident pg-cmdb pg-notify pg-ai pg-analytics

log_info "等待数据库就绪（30秒）..."
sleep 30

# 验证数据库健康
UNHEALTHY_DBS=""
for db in pg-log pg-alert pg-incident pg-cmdb pg-notify pg-ai pg-analytics; do
  status=$(docker compose ps "$db" --format "{{.Health}}" 2>/dev/null || echo "unknown")
  if [[ "$status" != "healthy" ]]; then
    UNHEALTHY_DBS="$UNHEALTHY_DBS $db"
  fi
done
if [[ -n "$UNHEALTHY_DBS" ]]; then
  log_warn "以下数据库尚未健康：$UNHEALTHY_DBS（继续等待或查看日志）"
  log_info "如需查看日志：docker compose logs pg-alert"
fi

# -------------------------------------------------------------------------
# Step 2: 启动中间件
# -------------------------------------------------------------------------
log_step "[2/7] 启动 Redis / Kafka / Elasticsearch / ClickHouse / MinIO"
docker compose up -d redis kafka elasticsearch clickhouse minio

log_info "等待 Kafka 启动（30秒）..."
sleep 30

# -------------------------------------------------------------------------
# Step 3: 初始化 Kafka Topics（幂等：已存在的 topic 会跳过）
# -------------------------------------------------------------------------
log_step "[3/7] 初始化 Kafka Topics"
cd "${PROJECT_ROOT}"
if bash ./deploy/scripts/init-kafka-topics.sh; then
  log_info "Kafka Topics 初始化完成"
else
  log_warn "Kafka Topics 初始化遇到问题，部分 Topic 可能已存在（可忽略）"
fi
cd "${DC_DIR}"

# -------------------------------------------------------------------------
# Step 4: 启动认证和网关
# -------------------------------------------------------------------------
log_step "[4/7] 启动 Keycloak / Kong"
docker compose up -d keycloak kong

log_info "等待 Keycloak 就绪（60秒）..."
WAIT=0
while [[ $WAIT -lt 60 ]]; do
  if curl -sf http://localhost:8080/health/ready &>/dev/null; then
    log_info "Keycloak 已就绪"
    break
  fi
  sleep 5
  WAIT=$((WAIT+5))
done

# -------------------------------------------------------------------------
# Step 5: 数据库迁移（幂等：已迁移的版本会跳过）
# -------------------------------------------------------------------------
log_step "[5/7] 运行数据库迁移"
if command -v migrate &>/dev/null; then
  cd "${PROJECT_ROOT}"
  if ./deploy/scripts/run-migrations.sh; then
    log_info "数据库迁移全部完成"
  else
    log_error "数据库迁移失败！请检查数据库连接和错误日志"
    log_info "可手动执行：./deploy/scripts/run-migrations.sh --service <svc-name>"
    exit 1
  fi
  cd "${DC_DIR}"
else
  log_warn "golang-migrate 未安装，跳过数据库迁移"
  log_info "请先安装：https://github.com/golang-migrate/migrate"
  log_info "然后手动执行：./deploy/scripts/run-migrations.sh"
fi

# -------------------------------------------------------------------------
# Step 6: 启动微服务
# -------------------------------------------------------------------------
log_step "[6/7] 启动 7 个微服务"
docker compose up -d \
  svc-log svc-alert svc-incident svc-cmdb \
  svc-notify svc-ai svc-analytics

log_info "等待微服务启动（30秒）..."
sleep 30

# 健康检查
log_info "检查微服务健康状态..."
SERVICES_OK=true
declare -A SVC_PORTS=([svc-log]=8081 [svc-alert]=8082 [svc-incident]=8083
                      [svc-cmdb]=8084 [svc-notify]=8085 [svc-ai]=8086 [svc-analytics]=8087)
for svc in "${!SVC_PORTS[@]}"; do
  port="${SVC_PORTS[$svc]}"
  if curl -sf "http://localhost:${port}/health" &>/dev/null; then
    log_info "✅ ${svc} (port ${port}): OK"
  else
    log_warn "⚠️  ${svc} (port ${port}): 未响应（可能还在启动中）"
    SERVICES_OK=false
  fi
done

# -------------------------------------------------------------------------
# Step 7: 启动监控
# -------------------------------------------------------------------------
log_step "[7/7] 启动 Prometheus / Grafana"
docker compose up -d prometheus grafana

# -------------------------------------------------------------------------
# 完成汇报
# -------------------------------------------------------------------------
echo ""
echo -e "${GREEN}=============================================${NC}"
echo -e "${GREEN}  OpsNexus 启动完成！${NC}"
echo -e "${GREEN}=============================================${NC}"
echo ""
echo "  API 网关:    http://localhost:8000"
echo "  Grafana:     http://localhost:3001  (admin/changeme)"
echo "  Keycloak:    http://localhost:8080/admin"
echo "  Prometheus:  http://localhost:9090"
echo ""
echo "  前端开发服务器（需单独启动）:"
echo "    cd frontend && pnpm install && pnpm dev"
echo "    访问: http://localhost:3000"
echo ""
if [[ "$SERVICES_OK" == "false" ]]; then
  echo -e "${YELLOW}  ⚠️  部分微服务可能还在启动，请稍候再检查：${NC}"
  echo "    for p in 8081 8082 8083 8084 8085 8086 8087; do"
  echo "      curl -s http://localhost:\$p/health; done"
fi
echo ""
