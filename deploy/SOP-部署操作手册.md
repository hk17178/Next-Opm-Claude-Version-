# OpsNexus 标准操作手册（SOP）

> 本手册面向首次部署人员，按步骤操作即可完成完整部署，无需预备知识。
> 每个步骤均提供期望输出，方便逐步核查。

---

## 选择你的部署方式

根据实际情况选择对应章节，按顺序执行即可。

```
┌─────────────────────────────────────────────────────────────────┐
│                     我有项目文件（在本地电脑上）                    │
│                                                                  │
│  ┌─────────────────────────┐   ┌──────────────────────────────┐ │
│  │  方式A：直接复制到服务器   │   │  方式B：先上传 GitLab，       │ │
│  │  无需 Git，最简单         │   │  再从 GitLab 克隆到服务器     │ │
│  │  → 看【第一章】           │   │  → 先看【第二章】             │ │
│  │                         │   │  → 再看【第三章】             │ │
│  └─────────────────────────┘   └──────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘

  方式C：生产环境（Kubernetes + Helm）→ 看【第四章】
```

---

## 目录

- [第一章：本地文件直接复制部署（推荐）](#第一章本地文件直接复制部署推荐)
- [第二章：上传代码到 GitLab](#第二章上传代码到-gitlab)
- [第三章：从 GitLab 克隆后部署](#第三章从-gitlab-克隆后部署)
- [第四章：生产环境部署（Kubernetes + Helm）](#第四章生产环境部署kubernetes--helm)
- [第五章：日常运维操作](#第五章日常运维操作)
- [第六章：常见错误与解决方案](#第六章常见错误与解决方案)
- [附录 A：端口速查表](#附录-a端口速查表)
- [附录 B：一键启动脚本说明](#附录-b一键启动脚本说明)
- [附录 C：上传代码到 GitHub](#附录-c上传代码到-github)

---

---

# 第一章：本地文件直接复制部署（推荐）

> **适用场景：** 项目文件在本地电脑，服务器可以联网拉取 Docker 镜像，
> 但不需要经过 GitLab，直接把文件传到服务器后拉起。
>
> **服务器要求：** Rocky Linux 8/9（或 Ubuntu 20.04+），
> 内存 ≥ 16GB，磁盘 ≥ 100GB，有 sudo 权限。

---

## 1.1 将项目文件传输到服务器

根据你的本地系统选择一种方式：

### Windows → Rocky Linux

**方法1：PowerShell 打包 + SCP 传输**

```powershell
# 在 Windows PowerShell 中执行

# 第一步：打包项目（替换路径为你的实际路径）
Compress-Archive -Path "D:\AI Project\Next Opm\*" -DestinationPath "C:\opsnexus.zip"

# 第二步：用 SCP 传到服务器（替换用户名和 IP）
scp C:\opsnexus.zip 用户名@192.168.1.100:/home/用户名/
```

**方法2：WinSCP 图形界面（更直观）**

1. 下载 WinSCP：https://winscp.net（免费）
2. 打开 WinSCP → 输入服务器 IP、用户名、密码 → 登录
3. 左侧找到 `D:\AI Project\Next Opm` 文件夹
4. 右侧切换到 `/home/用户名/`
5. 把左侧文件夹拖到右侧即可

---

### Mac / Linux → Rocky Linux

```bash
# 打包
zip -r /tmp/opsnexus.zip "/path/to/Next Opm"

# SCP 传输（替换用户名和 IP）
scp /tmp/opsnexus.zip 用户名@192.168.1.100:/home/用户名/

# 或用 rsync（大文件更快，支持断点续传）
rsync -avz --progress "/path/to/Next Opm/" 用户名@192.168.1.100:/home/用户名/opsnexus/
```

---

### U 盘传输（完全离线场景）

```bash
# 在 Rocky Linux 服务器上执行

# 查看 U 盘设备名
lsblk

# 挂载 U 盘（根据 lsblk 输出替换 sdb1）
sudo mkdir -p /mnt/udisk
sudo mount /dev/sdb1 /mnt/udisk

# 复制项目文件
cp -r /mnt/udisk/opsnexus /home/$USER/

# 卸载 U 盘
sudo umount /mnt/udisk
```

---

## 1.2 在 Rocky Linux 服务器上安装依赖

> SSH 登录服务器后，执行以下步骤。

**步骤 1：更新系统**

```bash
sudo dnf update -y
```

**步骤 2：安装 Docker**

```bash
# 添加 Docker 官方仓库
sudo dnf config-manager \
    --add-repo https://download.docker.com/linux/rhel/docker-ce.repo

# 安装 Docker + Compose 插件
sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

# 启动并设置开机自启
sudo systemctl enable --now docker

# 当前用户加入 docker 组（免 sudo）
sudo usermod -aG docker $USER
```

> ⚠️ **重要：执行完后必须退出并重新 SSH 登录，使 docker 组生效。**

```bash
exit
```

重新登录后验证：

```bash
docker run hello-world
```

✅ 期望输出最后包含：`Hello from Docker!`

---

**步骤 3：安装数据库迁移工具**

```bash
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xvz
sudo mv migrate /usr/local/bin/migrate
```

✅ 验证：`migrate -version` → 输出 `v4.17.0`

---

**步骤 4：配置系统参数（Elasticsearch 必须）**

```bash
sudo sysctl -w vm.max_map_count=262144
echo "vm.max_map_count=262144" | sudo tee -a /etc/sysctl.conf
```

---

**步骤 5：安装 unzip（如果传的是 zip 包）**

```bash
sudo dnf install -y unzip
```

---

## 1.3 解压项目

```bash
# 如果传的是 zip 包
unzip ~/opsnexus.zip -d ~/opsnexus

# 进入项目目录
cd ~/opsnexus
```

✅ 验证目录结构：

```bash
ls
```

应看到：`deploy/  docs/  frontend/  pkg/  services/  go.work  Makefile`

---

## 1.4 配置密码

```bash
# 创建配置文件
cp deploy/docker-compose/.env.example deploy/docker-compose/.env

# 编辑密码
nano deploy/docker-compose/.env
```

**必须修改以下 4 个密码（将 `changeme` 替换为强密码）：**

```
POSTGRES_PASSWORD=改成强密码
REDIS_PASSWORD=改成强密码
MINIO_ROOT_PASSWORD=改成强密码（至少8位）
KEYCLOAK_ADMIN_PASSWORD=改成强密码
```

> 如需接入 AI 功能，还需填写 API Key：
> ```
> CLAUDE_API_KEY=sk-ant-...
> ```

保存：`Ctrl+O` → `Enter` → `Ctrl+X`

---

## 1.5 一键启动

```bash
cd ~/opsnexus
chmod +x deploy/start-all.sh
./deploy/start-all.sh
```

脚本将自动按顺序完成（约 5–10 分钟）：

```
[1/7] 启动 7 个 PostgreSQL 数据库
[2/7] 启动 Redis / Kafka / Elasticsearch / ClickHouse / MinIO
[3/7] 初始化 Kafka Topics
[4/7] 启动 Keycloak / Kong API 网关
[5/7] 运行数据库迁移（建表）
[6/7] 构建并启动 7 个微服务
[7/7] 启动 Prometheus / Grafana 监控
```

---

## 1.6 开放防火墙端口（如已启用 firewalld）

```bash
sudo firewall-cmd --permanent --add-port=3000/tcp   # 前端
sudo firewall-cmd --permanent --add-port=3001/tcp   # Grafana
sudo firewall-cmd --permanent --add-port=8000/tcp   # Kong API 网关
sudo firewall-cmd --permanent --add-port=8080/tcp   # Keycloak
sudo firewall-cmd --reload
```

---

## 1.7 验证并访问

**健康检查：**

```bash
for port in 8081 8082 8083 8084 8085 8086 8087; do
  echo -n "Port $port: "
  curl -sf http://localhost:$port/health && echo "OK" || echo "FAIL"
done
```

✅ 所有端口返回 `OK` 表示部署成功。

**浏览器访问（替换为实际服务器 IP）：**

| 服务 | 地址 | 账号密码 |
|------|------|---------|
| Grafana 监控 | `http://服务器IP:3001` | admin / changeme |
| Keycloak 管理 | `http://服务器IP:8080/admin` | admin / 见 .env |
| Kong API 网关 | `http://服务器IP:8000` | — |
| Prometheus | `http://服务器IP:9090` | — |

---

## 1.8 本章命令汇总（可直接复制粘贴）

**第一段：安装依赖（需 root 或 sudo）**

```bash
sudo dnf update -y
sudo dnf config-manager --add-repo https://download.docker.com/linux/rhel/docker-ce.repo
sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin unzip
sudo systemctl enable --now docker
sudo usermod -aG docker $USER
sudo sysctl -w vm.max_map_count=262144
echo "vm.max_map_count=262144" | sudo tee -a /etc/sysctl.conf
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xvz
sudo mv migrate /usr/local/bin/migrate
echo "安装完成，请退出并重新 SSH 登录后继续执行第二段命令"
```

> ⚠️ 退出并重新登录后再执行下一段

**第二段：部署项目（重新登录后执行）**

```bash
# 解压（根据实际文件名修改）
unzip ~/opsnexus.zip -d ~/opsnexus
cd ~/opsnexus

# 配置密码
cp deploy/docker-compose/.env.example deploy/docker-compose/.env
nano deploy/docker-compose/.env   # 修改 changeme 为强密码

# 一键启动
chmod +x deploy/start-all.sh
./deploy/start-all.sh
```

---

---

# 第二章：上传代码到 GitLab

> **适用场景：** 项目代码目前在本地电脑，需要上传到 GitLab 仓库，
> 以便团队协作或后续从服务器 git clone 拉取部署。
>
> 如果你不需要 GitLab，直接跳过本章，看【第一章】即可。

---

## 2.1 在 GitLab 创建空白项目

1. 打开 GitLab（公司内网地址或 https://gitlab.com），登录
2. 点击右上角 **`+`** → **New project**
3. 选择 **Create blank project**
4. 填写信息：
   - **Project name**：`opsnexus`
   - **Namespace**：选择你的组织（如 `platform`）
   - **Visibility Level**：Private（推荐）
5. ⚠️ **取消勾选** "Initialize repository with a README"
6. 点击 **Create project**

创建成功后，页面显示仓库地址，复制 **HTTPS** 地址备用：

```
https://gitlab.company.com/platform/opsnexus.git
```

---

## 2.2 安装并配置 Git（首次使用需执行）

```bash
# 设置姓名和邮箱（会出现在提交记录中）
git config --global user.name "你的姓名"
git config --global user.email "你的邮箱@company.com"
```

✅ 验证：

```bash
git config --global --list
# 期望看到：user.name=xxx 和 user.email=xxx
```

---

## 2.3 初始化并上传代码

```bash
# 进入项目目录（Windows 路径示例）
cd "D:\AI Project\Next Opm"

# 初始化 Git 仓库
git init

# 确认 .env 不会被提交（项目已配置 .gitignore，应已排除）
git status | grep .env
# 如果看到 .env，执行：git rm --cached deploy/docker-compose/.env

# 添加所有文件到暂存区
git add .

# 创建初始提交
git commit -m "feat: initial commit - OpsNexus platform"

# 关联 GitLab 远程仓库（替换为 2.1 步复制的地址）
git remote add origin https://gitlab.company.com/platform/opsnexus.git

# 推送到 GitLab
git push -u origin main
```

> **推送时会要求输入 GitLab 用户名和密码。**
> GitLab 2021 年后不支持直接用账号密码，需使用 Personal Access Token：
>
> GitLab → 右上角头像 → Edit Profile → Access Tokens
> → 点击 Add new token → 勾选 `write_repository` → 创建
> → 复制 Token，推送时当作密码使用

✅ 推送成功期望输出：

```
To https://gitlab.company.com/platform/opsnexus.git
 * [new branch]      main -> main
Branch 'main' set up to track remote branch 'main' from 'origin'.
```

---

## 2.4 配置 SSH Key（可选，免密推送）

```bash
# 生成密钥（一路回车使用默认）
ssh-keygen -t ed25519 -C "你的邮箱@company.com"

# 查看公钥
cat ~/.ssh/id_ed25519.pub
```

将公钥内容复制，然后：
GitLab → Edit Profile → SSH Keys → Add key → 粘贴 → 保存

```bash
# 改用 SSH 地址推送（之后免密）
git remote set-url origin git@gitlab.company.com:platform/opsnexus.git
git push origin main
```

---

## 2.5 后续代码更新推送

每次修改后执行：

```bash
git add .
git commit -m "描述本次改动的内容"
git push
```

---

---

# 第三章：从 GitLab 克隆后部署

> **适用场景：** 代码已上传到 GitLab（见第二章），
> 服务器能访问 GitLab，通过 git clone 拉取代码后部署。

---

## 3.1 在服务器上安装依赖

与第一章 1.2 节相同，执行 Docker、migrate、系统参数的安装。
此处为快速命令：

```bash
# 安装 Docker（Rocky Linux）
sudo dnf config-manager --add-repo https://download.docker.com/linux/rhel/docker-ce.repo
sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
sudo systemctl enable --now docker
sudo usermod -aG docker $USER
sudo sysctl -w vm.max_map_count=262144
echo "vm.max_map_count=262144" | sudo tee -a /etc/sysctl.conf
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xvz
sudo mv migrate /usr/local/bin/migrate
```

> ⚠️ 执行完后退出重新登录。

---

## 3.2 克隆代码

```bash
# 替换为你的 GitLab 仓库地址
git clone https://gitlab.company.com/platform/opsnexus.git
cd opsnexus
```

✅ 验证：`ls` 应看到 `deploy/  services/  frontend/` 等目录

---

## 3.3 配置密码并启动

```bash
cp deploy/docker-compose/.env.example deploy/docker-compose/.env
nano deploy/docker-compose/.env    # 修改 4 个 changeme 密码

chmod +x deploy/start-all.sh
./deploy/start-all.sh
```

---

## 3.4 后续更新部署

```bash
cd ~/opsnexus

# 拉取最新代码
git pull

# 重新构建并启动微服务（基础设施不需要重启）
cd deploy/docker-compose
docker compose up -d --build \
  svc-log svc-alert svc-incident svc-cmdb \
  svc-notify svc-ai svc-analytics
```

---

---

# 第四章：生产环境部署（Kubernetes + Helm）

> **适用场景：** 正式生产环境，需要高可用、弹性伸缩。
> 需要已有 Kubernetes 集群（EKS/GKE/AKS 或自建）。

---

## 4.1 前置条件

- Kubernetes ≥ 1.28
- Helm ≥ 3.13
- 外部托管服务（推荐）：
  - PostgreSQL → AWS RDS / 阿里云 RDS
  - Redis → AWS ElastiCache / 阿里云 Redis
  - Kafka → AWS MSK / 阿里云 Kafka
- 镜像仓库已配置：`registry.company.com/opsnexus`

---

## 4.2 构建并推送镜像

GitLab CI 在 push 到 `main` 分支时自动构建。手动构建：

```bash
IMAGE_TAG=$(git rev-parse --short HEAD)
for svc in svc-log svc-alert svc-incident svc-cmdb svc-notify svc-ai svc-analytics; do
  docker build -t registry.company.com/opsnexus/${svc}:${IMAGE_TAG} \
    -f services/${svc}/Dockerfile .
  docker push registry.company.com/opsnexus/${svc}:${IMAGE_TAG}
done
```

---

## 4.3 配置 Helm Values

```bash
cp deploy/helm/opsnexus/values-staging.yaml my-values.yaml
nano my-values.yaml
```

关键配置项（覆盖默认值）：

```yaml
global:
  domain: ops.company.com
  image:
    registry: registry.company.com/opsnexus
    tag: "abc1234"              # 替换为实际 Git SHA

database:
  host: "rds-opsnexus.xxxx.amazonaws.com"

kafka:
  bootstrapServers: "broker1:9092,broker2:9092"

redis:
  url: "redis://:密码@elasticache.xxxx.amazonaws.com:6379/0"
```

---

## 4.4 部署

```bash
# Staging 环境
helm upgrade --install opsnexus deploy/helm/opsnexus \
  -f deploy/helm/opsnexus/values-staging.yaml \
  -f my-values.yaml \
  --namespace opsnexus --create-namespace \
  --wait --timeout 10m

# 生产环境（atomic：失败自动回滚）
helm upgrade --install opsnexus deploy/helm/opsnexus \
  -f deploy/helm/opsnexus/values-prod.yaml \
  -f my-values-prod.yaml \
  --namespace opsnexus \
  --atomic --timeout 15m
```

---

## 4.5 验证

```bash
# 查看所有 Pod 状态（应全部为 Running）
kubectl get pods -n opsnexus

# 查看服务
kubectl get svc -n opsnexus

# 查看日志
kubectl logs -n opsnexus -l app=svc-alert -f

# 滚动更新（只改镜像 tag）
helm upgrade opsnexus deploy/helm/opsnexus \
  -n opsnexus --reuse-values \
  --set global.image.tag=<new-tag>

# 回滚
helm rollback opsnexus -n opsnexus
```

---

## 4.6 生产副本数（HPA 自动扩缩）

| 服务 | 最小副本 | 最大副本 | 触发 CPU |
|------|---------|---------|---------|
| svc-log | 3 | 10 | 70% |
| svc-alert | 3 | 8 | 70% |
| svc-incident | 2 | 6 | 70% |
| svc-cmdb | 2 | 6 | 70% |
| svc-notify | 2 | 8 | 70% |
| svc-ai | 2 | 6 | 70% |
| svc-analytics | 2 | 6 | 70% |

---

---

# 第五章：日常运维操作

## 查看服务状态和日志

```bash
cd deploy/docker-compose

# 查看所有容器状态
docker compose ps

# 实时查看某服务日志
docker compose logs -f svc-alert

# 查看最近 100 行
docker compose logs --tail=100 svc-alert
```

## 重启服务

```bash
# 重启单个服务
docker compose restart svc-alert

# 重新构建并重启（代码有更新时）
docker compose up -d --build svc-alert
```

## 停止和清理

```bash
# 停止全部（保留数据）
docker compose down

# 停止并删除所有数据卷（⚠️ 危险，数据全部清空）
docker compose down -v
```

## 数据库备份

```bash
# 备份
docker compose exec pg-alert \
  pg_dump -U opsnexus opm_alert \
  > backup_opm_alert_$(date +%Y%m%d_%H%M%S).sql

# 恢复
docker compose exec -T pg-alert \
  psql -U opsnexus opm_alert \
  < backup_opm_alert_20240101_120000.sql
```

## 配置 Keycloak（首次）

1. 浏览器打开 `http://服务器IP:8080/admin`
2. 账号 `admin`，密码见 `.env` 中 `KEYCLOAK_ADMIN_PASSWORD`
3. 左上角切换到 `opsnexus` Realm（应已自动导入）
4. 左侧 **Users** → **Add User** → 创建管理员账号
5. 进入该用户 → **Credentials** → 设置密码（关闭 Temporary）
6. 进入 **Role Mappings** → 分配 `opsnexus-admin` 角色

## 前端开发服务器

```bash
cd frontend
pnpm install
pnpm run dev      # 访问 http://localhost:3000
```

---

---

# 第六章：常见错误与解决方案

## 错误 1：PostgreSQL 容器 unhealthy

**现象：** `docker compose ps` 显示 `unhealthy`

**解决：**
```bash
docker compose logs pg-alert | tail -20  # 查看错误
docker compose down
docker volume rm opsnexus_pg-alert-data  # 清空重建
docker compose up -d pg-alert
```

---

## 错误 2：数据库迁移失败（dirty state）

**现象：** `error: Dirty database version 1`

**解决：**
```bash
migrate \
  -path services/svc-alert/migrations \
  -database "postgres://opsnexus:密码@localhost:5434/opm_alert?sslmode=disable" \
  force 1
./deploy/scripts/run-migrations.sh --service svc-alert
```

---

## 错误 3：Elasticsearch 启动失败

**现象：** `vm.max_map_count [65530] is too low`

**解决：**
```bash
sudo sysctl -w vm.max_map_count=262144
```

---

## 错误 4：Kafka 无法连接

**现象：** 服务日志有 `LEADER_NOT_AVAILABLE`

**解决：**
```bash
# 等待更长时间
sleep 30
./deploy/scripts/init-kafka-topics.sh

# 若仍失败，清空重建
docker compose stop kafka
docker volume rm opsnexus_kafka-data
docker compose up -d kafka
sleep 30
./deploy/scripts/init-kafka-topics.sh
```

---

## 错误 5：端口被占用

**现象：** `bind: address already in use`

**解决：**
```bash
sudo lsof -i :8082      # 查找占用进程
sudo kill -9 <PID>      # 结束进程
docker compose up -d svc-alert
```

---

## 错误 6：Keycloak 无 opsnexus Realm

**现象：** 登录后看不到 opsnexus Realm

**解决：**
```bash
docker compose exec keycloak \
  /opt/keycloak/bin/kc.sh import \
  --file /opt/keycloak/data/import/opsnexus-realm.json
```

---

## 错误 7：Docker 命令需要 sudo

**现象：** 不加 sudo 提示 `permission denied`

**解决：**
```bash
sudo usermod -aG docker $USER
exit   # 必须重新登录
```

---

---

# 附录 A：端口速查表

| 服务 | 端口 | 说明 |
|------|------|------|
| OpsNexus 前端 | 3000 | React 开发服务器 |
| Grafana | 3001 | 监控面板 |
| PostgreSQL svc-log | 5433 | 数据库（调试直连）|
| PostgreSQL svc-alert | 5434 | 数据库（调试直连）|
| PostgreSQL svc-incident | 5435 | 数据库（调试直连）|
| PostgreSQL svc-cmdb | 5436 | 数据库（调试直连）|
| PostgreSQL svc-notify | 5437 | 数据库（调试直连）|
| PostgreSQL svc-ai | 5438 | 数据库（调试直连）|
| PostgreSQL svc-analytics | 5439 | 数据库（调试直连）|
| Redis | 6379 | 缓存 |
| Kafka | 9092 | 消息总线 |
| Elasticsearch | 9200 | 日志检索 |
| ClickHouse HTTP | 8123 | 时序数据 |
| MinIO API | 9010 | 对象存储 |
| MinIO 控制台 | 9001 | 对象存储 Web UI |
| Keycloak | 8080 | SSO 认证 |
| Kong Gateway | 8000 | API 入口（HTTP）|
| Kong Gateway TLS | 8443 | API 入口（HTTPS）|
| Kong Admin API | 8001 | Kong 管理 |
| Prometheus | 9090 | 指标采集 |
| svc-log HTTP/gRPC | 8081 / 9085 | 日志服务 |
| svc-alert HTTP/gRPC | 8082 / 9086 | 告警服务 |
| svc-incident HTTP/gRPC | 8083 / 9083 | 事件服务 |
| svc-cmdb HTTP/gRPC | 8084 / 9084 | CMDB 服务 |
| svc-notify HTTP/gRPC | 8085 / 9082 | 通知服务 |
| svc-ai HTTP/gRPC | 8086 / 9081 | AI 服务 |
| svc-analytics HTTP/gRPC | 8087 / 9087 | 分析服务 |

---

# 附录 B：一键启动脚本说明

脚本位置：`deploy/start-all.sh`

**使用方式：**

```bash
chmod +x deploy/start-all.sh
./deploy/start-all.sh
```

**脚本特性：**

- ✅ 幂等运行：重复执行不会破坏已运行的服务
- ✅ 前置检查：启动前验证 Docker、.env 文件是否就绪
- ✅ 健康等待：每步等待就绪后再进行下一步
- ✅ 微服务健康检查：启动后自动检测 7 个服务是否响应
- ✅ 彩色输出：绿色=正常，黄色=警告，红色=错误

**脚本执行顺序：**

```
[1/7] PostgreSQL × 7  ──→ 等待 30s
[2/7] Redis + Kafka + ES + ClickHouse + MinIO
[3/7] 初始化 Kafka Topics  ──→ 等待 Kafka 就绪
[4/7] Keycloak + Kong  ──→ 等待 Keycloak /health/ready
[5/7] 数据库迁移  ──→ golang-migrate up
[6/7] 7 个微服务构建启动  ──→ 等待 30s
[7/7] Prometheus + Grafana
```

---

---

# 附录 C：上传代码到 GitHub

> **适用场景：** 项目代码在本地电脑，需要上传到 GitHub 仓库，
> 以便个人备份、开源共享或团队协作。
>
> 与 GitLab 操作非常相似，主要差异在于认证方式（使用 Personal Access Token）
> 和网站界面不同。

---

## C.1 注册并登录 GitHub

1. 打开 https://github.com，点击右上角 **Sign up** 注册账号（已有账号直接登录）
2. 登录后，点击右上角头像 → **Your repositories** 可看到你的仓库列表

---

## C.2 创建空白仓库

1. 点击右上角 **`+`** → **New repository**
2. 填写信息：
   - **Repository name**：`opsnexus`
   - **Description**（可选）：`OpsNexus AIOps Platform`
   - **Visibility**：
     - `Private`（私有，推荐）
     - `Public`（公开，所有人可见）
3. ⚠️ **不要勾选** "Add a README file"、"Add .gitignore"、"Choose a license"
   （因为本地已有这些文件，勾选会导致推送冲突）
4. 点击 **Create repository**

创建成功后，页面显示仓库地址，复制 **HTTPS** 地址备用：

```
https://github.com/你的用户名/opsnexus.git
```

---

## C.3 生成 Personal Access Token（必须）

> GitHub 从 2021 年 8 月起已禁止用账号密码推送代码，必须使用 Token。

1. 右上角头像 → **Settings**
2. 左侧最底部 → **Developer settings**
3. 左侧 → **Personal access tokens** → **Tokens (classic)**
4. 点击 **Generate new token** → **Generate new token (classic)**
5. 填写：
   - **Note**：`opsnexus-push`（备注用途）
   - **Expiration**：`90 days`（或选 No expiration）
   - **勾选权限**：`repo`（勾选整个 repo 大类即可，含读写权限）
6. 点击 **Generate token**
7. ⚠️ **立即复制 Token**（页面关闭后无法再次查看！）

Token 格式示例：`ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`

---

## C.4 安装并配置 Git（首次使用需执行）

```bash
# 设置姓名和邮箱（会出现在提交记录中）
git config --global user.name "你的姓名"
git config --global user.email "你的邮箱@gmail.com"
```

✅ 验证：

```bash
git config --global --list
# 期望看到：user.name=xxx 和 user.email=xxx
```

---

## C.5 初始化并上传代码

```bash
# 进入项目目录
cd "D:\AI Project\Next Opm"       # Windows
# cd "/path/to/Next Opm"          # Mac/Linux

# 初始化 Git 仓库（如果还没有 .git 目录）
git init

# 确认 .env 不会被提交（项目已配置 .gitignore，会自动排除）
git status | grep .env
# 如果意外出现，执行：git rm --cached deploy/docker-compose/.env

# 添加所有文件到暂存区
git add .

# 查看将要提交的文件列表（可选，建议检查一遍）
git status

# 创建初始提交
git commit -m "feat: initial commit - OpsNexus platform"

# 关联 GitHub 远程仓库（替换为 C.2 步复制的地址）
git remote add origin https://github.com/你的用户名/opsnexus.git

# 将本地分支重命名为 main（GitHub 默认分支名）
git branch -M main

# 推送到 GitHub
git push -u origin main
```

**推送时弹出用户名密码提示：**

```
Username for 'https://github.com': 你的GitHub用户名
Password for 'https://...':        粘贴 C.3 步生成的 Token（不是账号密码！）
```

✅ 推送成功期望输出：

```
Enumerating objects: 312, done.
Writing objects: 100% (312/312), done.
To https://github.com/你的用户名/opsnexus.git
 * [new branch]      main -> main
Branch 'main' set up to track remote branch 'main' from 'origin'.
```

---

## C.6 让 Git 记住 Token（避免每次输入）

### 方法一：让 Git 缓存凭证（推荐）

```bash
# 缓存 15 分钟（默认）
git config --global credential.helper cache

# 或缓存更长时间（单位：秒，86400 = 1天）
git config --global credential.helper 'cache --timeout=86400'
```

### 方法二：Windows 凭据管理器（Windows 系统推荐）

```bash
git config --global credential.helper manager
```

Windows 会弹出一个登录框，输入一次后自动保存，后续无需再输入。

### 方法三：直接写入 URL（不推荐，有安全风险）

```bash
# 将 Token 嵌入地址（注意：Token 明文存储）
git remote set-url origin https://你的用户名:ghp_xxxxxx@github.com/你的用户名/opsnexus.git
```

---

## C.7 配置 SSH Key（可选，完全免密）

SSH Key 是更安全、更方便的认证方式，配置一次后永久有效。

**第一步：生成密钥对**

```bash
ssh-keygen -t ed25519 -C "你的邮箱@gmail.com"
# 一路回车（不设置 passphrase 最方便）
```

生成的文件：
- 私钥：`~/.ssh/id_ed25519`（保密，不要泄露）
- 公钥：`~/.ssh/id_ed25519.pub`（上传到 GitHub）

**第二步：查看公钥**

```bash
# Linux/Mac
cat ~/.ssh/id_ed25519.pub

# Windows PowerShell
cat $HOME\.ssh\id_ed25519.pub
```

复制输出的全部内容（以 `ssh-ed25519 AAAA...` 开头）。

**第三步：添加到 GitHub**

1. GitHub → 右上角头像 → **Settings**
2. 左侧 → **SSH and GPG keys**
3. 点击 **New SSH key**
4. **Title**：`my-laptop`（随意填，标识这台电脑）
5. **Key**：粘贴公钥内容
6. 点击 **Add SSH key**

**第四步：测试连接**

```bash
ssh -T git@github.com
# 期望输出：Hi 你的用户名! You've successfully authenticated...
```

**第五步：改用 SSH 地址推送（之后永久免密）**

```bash
git remote set-url origin git@github.com:你的用户名/opsnexus.git
git push origin main
```

---

## C.8 后续代码更新推送

每次修改代码后执行：

```bash
# 查看改了哪些文件
git status

# 添加并提交
git add .
git commit -m "fix: 描述本次修改内容"

# 推送到 GitHub
git push
```

---

## C.9 常用 Git 操作速查

| 操作 | 命令 |
|------|------|
| 查看提交历史 | `git log --oneline` |
| 撤销未提交的修改 | `git restore 文件名` |
| 撤销最后一次提交（保留修改） | `git reset --soft HEAD~1` |
| 创建并切换分支 | `git checkout -b feature/xxx` |
| 合并分支到 main | `git checkout main && git merge feature/xxx` |
| 查看远程地址 | `git remote -v` |
| 从 GitHub 拉取最新代码 | `git pull` |
| 克隆仓库到新机器 | `git clone https://github.com/你的用户名/opsnexus.git` |

---

## C.10 GitHub vs GitLab 差异对照

| 对比项 | GitHub | GitLab |
|--------|--------|--------|
| 地址 | https://github.com | 公司内网或 https://gitlab.com |
| 认证 Token 名称 | Personal Access Token (PAT) | Personal Access Token |
| Token 权限选项 | `repo`（全部读写） | `write_repository` |
| SSH Key 设置位置 | Settings → SSH and GPG keys | Edit Profile → SSH Keys |
| 默认分支名 | `main` | `main`（旧版本可能是 `master`）|
| CI/CD | GitHub Actions | GitLab CI/CD |
| 推送命令 | 完全相同 | 完全相同 |
