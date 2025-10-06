#!/usr/bin/env bash
set -euo pipefail

# =============================
# Configurações
# =============================

# Cores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

LOG_FILE="/var/log/autohunting_install.log"
DELAY_MS=300
DRY_RUN=${DRY_RUN:-0}
TOOLS_PREFIX=${TOOLS_PREFIX:-/opt/autohunting}
GOINSTALL_BIN=${GOINSTALL_BIN:-$(command -v go || true)}
mkdir -p "$TOOLS_PREFIX" >/dev/null 2>&1 || true
TOOLS_PM_INSTALLED=()
TOOLS_GIT_INSTALLED=()
GIT_INSTALL_DIR=""  # diretório personalizado fornecido pelo usuário
MAX_JOBS=4  # para parallelização de instalações

# =============================
# Funções de verificação
# =============================

verifica_root() {
    if [ "$(id -u)" -ne 0 ] && [ -z "${SUDO_USER:-}" ]; then
        log "ERROR" "Erro: Execute como root (sudo)!"
        echo -e "${RED}Erro: Execute como root (sudo)!${NC}"
        exit 1
    fi
    log "INFO" "Permissões ok (root or sudo)."
}

show_menu(){
    echo -e "Ferramenta de configuração de ambiente"
    echo "1) Declarar local da pasta de informações SUJAS"
    echo "2) Declarar local da pasta de informações LIMPAS"
    echo "3) Declarar local da pasta de LOGS"
    echo "4) Declarar localização da pasta de JSONs"
    echo "5) Configurar pasta de wordlists"
    echo "6) Configurar serviço de inicialização automática"
    echo "7) Mostrar configuração atual"
    echo "0) Voltar"
}