#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# OpsNexus Host Preparation Script
# Supports: Ubuntu/Debian, Rocky/RHEL
# ============================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root (or with sudo)."
        exit 1
    fi
}

detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS_ID="${ID}"
        OS_VERSION="${VERSION_ID}"
    else
        log_error "Cannot detect OS. /etc/os-release not found."
        exit 1
    fi

    case "${OS_ID}" in
        ubuntu|debian)
            PKG_MANAGER="apt"
            ;;
        rocky|rhel|centos|almalinux)
            PKG_MANAGER="yum"
            ;;
        *)
            log_error "Unsupported OS: ${OS_ID}"
            exit 1
            ;;
    esac

    log_info "Detected OS: ${OS_ID} ${OS_VERSION} (package manager: ${PKG_MANAGER})"
}

# ============================================================
# Install Docker CE + Docker Compose Plugin
# ============================================================
install_docker() {
    if command -v docker &>/dev/null; then
        log_info "Docker already installed: $(docker --version)"
    else
        log_info "Installing Docker CE..."

        if [[ "${PKG_MANAGER}" == "apt" ]]; then
            apt-get update -y
            apt-get install -y ca-certificates curl gnupg lsb-release

            install -m 0755 -d /etc/apt/keyrings
            curl -fsSL "https://download.docker.com/linux/${OS_ID}/gpg" | \
                gpg --dearmor -o /etc/apt/keyrings/docker.gpg
            chmod a+r /etc/apt/keyrings/docker.gpg

            echo \
              "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
              https://download.docker.com/linux/${OS_ID} \
              $(lsb_release -cs) stable" > /etc/apt/sources.list.d/docker.list

            apt-get update -y
            apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

        elif [[ "${PKG_MANAGER}" == "yum" ]]; then
            yum install -y yum-utils
            yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
            yum install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
        fi

        systemctl enable --now docker
        log_info "Docker installed successfully."
    fi

    # Ensure docker compose plugin is available
    if docker compose version &>/dev/null; then
        log_info "Docker Compose plugin: $(docker compose version)"
    else
        log_error "Docker Compose plugin not found. Please install it manually."
        exit 1
    fi
}

# ============================================================
# System Parameters
# ============================================================
configure_sysctl() {
    log_info "Configuring system parameters..."

    local SYSCTL_FILE="/etc/sysctl.d/99-opsnexus.conf"

    cat > "${SYSCTL_FILE}" <<'SYSCTL'
# Elasticsearch requires vm.max_map_count >= 262144
vm.max_map_count=262144

# Network tuning
net.core.somaxconn=65535
net.ipv4.tcp_max_syn_backlog=65535
net.ipv4.ip_local_port_range=1024 65535

# File descriptors
fs.file-max=1048576
SYSCTL

    sysctl --system
    log_info "System parameters configured."
}

# ============================================================
# Data Directories
# ============================================================
create_data_dirs() {
    log_info "Creating data directories..."

    local BASE_DIR="/opt/opsnexus/data"
    local DIRS=(
        "${BASE_DIR}/pg-log"
        "${BASE_DIR}/pg-alert"
        "${BASE_DIR}/pg-incident"
        "${BASE_DIR}/pg-cmdb"
        "${BASE_DIR}/pg-notify"
        "${BASE_DIR}/pg-ai"
        "${BASE_DIR}/pg-analytics"
        "${BASE_DIR}/redis"
        "${BASE_DIR}/kafka"
        "${BASE_DIR}/elasticsearch"
        "${BASE_DIR}/clickhouse"
        "${BASE_DIR}/minio"
    )

    for dir in "${DIRS[@]}"; do
        mkdir -p "${dir}"
    done

    # Elasticsearch needs specific UID
    chown -R 1000:1000 "${BASE_DIR}/elasticsearch"

    log_info "Data directories created at ${BASE_DIR}."
}

# ============================================================
# Optional: kubectl + helm
# ============================================================
install_kubectl() {
    if command -v kubectl &>/dev/null; then
        log_info "kubectl already installed: $(kubectl version --client --short 2>/dev/null || true)"
        return
    fi

    log_info "Installing kubectl..."
    curl -fsSL -o /usr/local/bin/kubectl \
        "https://dl.k8s.io/release/$(curl -fsSL https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    chmod +x /usr/local/bin/kubectl
    log_info "kubectl installed."
}

install_helm() {
    if command -v helm &>/dev/null; then
        log_info "helm already installed: $(helm version --short 2>/dev/null || true)"
        return
    fi

    log_info "Installing Helm..."
    curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
    log_info "Helm installed."
}

# ============================================================
# Main
# ============================================================
main() {
    check_root
    detect_os

    install_docker
    configure_sysctl
    create_data_dirs

    # Optional tools — pass --with-k8s to install
    if [[ "${1:-}" == "--with-k8s" ]]; then
        install_kubectl
        install_helm
    else
        log_info "Skipping kubectl/helm install. Pass --with-k8s to include them."
    fi

    log_info "Host preparation complete."
}

main "$@"
