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

# =============================
# Pacotes de ferramentas 
# =============================
BASE_PACKAGES=(
  curl jq git nmap
)

RECON_PACKAGES=(
  subfinder amass assetfinder katana httpx gau waybackurls reconftw bugbountytoolkit awsbucketdump
)

SCANNER_PACKAGES=(
  nuclei ffuf gobuster nikto sqlmap wafw00f dalfox xsstrike
)

WEB_TOOLS=(
  gopherus lfisuite fimap oralyzer cmseek
)

AUX_RECON=(
  ffuf kiterunner gau waybackurls awsbucketdump aquatone eyewitness
)

MISC_TOOLS=(
  linkfinder gf postman gitdumper metasploit
)

# =============================
# Helpers e logging
# =============================
log_message() {
    local level="$1"; shift
    local msg="$*"
    echo -e "[$(date '+%F %T')] ${level}: $msg"
}

# =============================
# Detect & package manager mount
# =============================
# Sets CMD_PACK_MANAGER_INSTALL and CMD_PACK_MANAGER_NAME (without sudo if running as root)
detect_package_manager() {
    local prefix=""
    if [ "$(id -u)" -ne 0 ]; then
        # not root -> prefer sudo
        prefix="sudo "
    fi

    if command -v apt >/dev/null 2>&1; then
        CMD_PACK_MANAGER_INSTALL="${prefix}apt install -y"
        CMD_PACK_MANAGER_NAME="apt"
    elif command -v pacman >/dev/null 2>&1; then
        # pacman non-interactive
        CMD_PACK_MANAGER_INSTALL="${prefix}pacman -S --noconfirm"
        CMD_PACK_MANAGER_NAME="pacman"
    elif command -v dnf >/dev/null 2>&1; then
        CMD_PACK_MANAGER_INSTALL="${prefix}dnf install -y"
        CMD_PACK_MANAGER_NAME="dnf"
    elif command -v yum >/dev/null 2>&1; then
        CMD_PACK_MANAGER_INSTALL="${prefix}yum install -y"
        CMD_PACK_MANAGER_NAME="yum"
    else
        log_message "ERROR" "Gerenciador de pacotes não detectado. Instale apt/dnf/pacman ou edite o script."
        exit 1
    fi
    log_message "SUCCESS" "Gerenciador de pacotes detectado: $CMD_PACK_MANAGER_NAME (cmd: $CMD_PACK_MANAGER_INSTALL)"
}

# Exec install genérico (usa CMD_PACK_MANAGER_INSTALL)
_exec_install() {
    local pkg="$1"
    local cmdline="${CMD_PACK_MANAGER_INSTALL} ${pkg}"
    log_message "INFO" "Executando: $cmdline"
    if [ "${DRY_RUN:-0}" -eq 1 ]; then
        log_message "DRY" "(simulação) $cmdline"
        return 0
    fi
    if eval "$cmdline"; then
        log_message "SUCCESS" "Instalado: $pkg"
        return 0
    else
        log_message "ERROR" "Falhou ao instalar: $pkg"
        return 1
    fi
}

# =============================
# Map package -> install method (go / pip / git / package manager fallback)
# =============================
get_install_cmd() {
    local pkg="$1"
    case "$pkg" in
        # ProjectDiscovery tools (Go)
        subfinder) echo "GO111MODULE=on ${GOINSTALL_BIN} install github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest" ;;
        nuclei) echo "GO111MODULE=on ${GOINSTALL_BIN} install github.com/projectdiscovery/nuclei/v2/cmd/nuclei@latest" ;;
        httpx) echo "GO111MODULE=on ${GOINSTALL_BIN} install github.com/projectdiscovery/httpx/cmd/httpx@latest" ;;
        assetfinder) echo "GO111MODULE=on ${GOINSTALL_BIN} install github.com/tomnomnom/assetfinder@latest" ;;
        gau) echo "GO111MODULE=on ${GOINSTALL_BIN} install github.com/lc/gau/v2/cmd/gau@latest" ;;
        waybackurls) echo "GO111MODULE=on ${GOINSTALL_BIN} install github.com/tomnomnom/waybackurls@latest" ;;
        ffuf) echo "GO111MODULE=on ${GOINSTALL_BIN} install github.com/ffuf/ffuf@latest" ;;
        gobuster) echo "GO111MODULE=on ${GOINSTALL_BIN} install github.com/OJ/gobuster/v3@latest" ;;
        dalfox) echo "GO111MODULE=on ${GOINSTALL_BIN} install github.com/hahwul/dalfox/v2@latest" ;;
        aquatone) echo "GO111MODULE=on ${GOINSTALL_BIN} install github.com/michenriksen/aquatone@latest" ;;
        httpx) echo "GO111MODULE=on ${GOINSTALL_BIN} install github.com/projectdiscovery/httpx/cmd/httpx@latest" ;;
        # pip / python tools
        linkfinder) echo "pip3 install linkfinder || true" ;;
        wafw00f) echo "pip3 install wafw00f || true" ;;
        xsstrike) echo "pip3 install xsstrike || true" ;;
        # git clones (no build)
        katana) echo "git clone https://github.com/projectdiscovery/katana.git ${TOOLS_PREFIX}/katana || true" ;;
        reconftw) echo "git clone https://github.com/six2dez/reconftw.git ${TOOLS_PREFIX}/reconftw || true" ;;
        eyewitness) echo "git clone https://github.com/FortyNorthSecurity/EyeWitness.git ${TOOLS_PREFIX}/EyeWitness || true" ;;
        # nuclei templates
        "nuclei-templates") echo "git clone https://github.com/projectdiscovery/nuclei-templates.git ${TOOLS_PREFIX}/nuclei-templates || true" ;;
        # fallback empty -> use package manager
        *) echo "" ;;
    esac
}

# =============================
# Map package -> expected binary names
# =============================
pkg_binary_candidates() {
    local pkg="$1"
    case "$pkg" in
        metasploit) echo "msfconsole" ;;
        postman) echo "postman" ;;
        sqlmap) echo "sqlmap" ;;
        wafw00f) echo "wafw00f" ;;
        subfinder) echo "subfinder" ;;
        nuclei) echo "nuclei" ;;
        httpx) echo "httpx" ;;
        assetfinder) echo "assetfinder" ;;
        gau) echo "gau" ;;
        waybackurls) echo "waybackurls" ;;
        ffuf) echo "ffuf" ;;
        gobuster) echo "gobuster" ;;
        dalfox) echo "dalfox" ;;
        aquatone) echo "aquatone" ;;
        eyewitness) echo "EyeWitness EyeWitness.py eyewitness" ;;
        linkfinder) echo "linkfinder" ;;
        gf) echo "gf" ;;
        kiterunner) echo "kiterunner" ;;
        amass) echo "amass" ;;
        *) echo "$pkg" ;;
    esac
}

# =============================
# is_installed improved
# =============================
is_installed() {
    local pkg="$1"
    # 1) try known binary names (multiple candidates)
    local candidates
    candidates="$(pkg_binary_candidates "$pkg")"
    for c in $candidates; do
        if command -v "$c" >/dev/null 2>&1; then
            return 0
        fi
    done

    # 2) try some heuristics: lowercase, dash/underscore swaps
    local lower="${pkg,,}"
    for c in "$pkg" "${lower}" "${pkg//_/-}" "${pkg//-/_}"; do
        if command -v "$c" >/dev/null 2>&1; then
            return 0
        fi
    done

    # 3) fallback to package DB check per distro (best-effort)
    case "${CMD_PACK_MANAGER_NAME-}" in
        apt)
            if dpkg -s "$pkg" >/dev/null 2>&1; then return 0; fi
            ;;
        pacman)
            if pacman -Qi "$pkg" >/dev/null 2>&1; then return 0; fi
            ;;
        dnf|yum)
            if rpm -q "$pkg" >/dev/null 2>&1; then return 0; fi
            ;;
    esac

    return 1
}

# =============================
# install helpers
# =============================
install_one() {
    local pkg="$1"
    # skip if present
    if is_installed "$pkg"; then
        log_message "INFO" "Pulando (já instalado): $pkg"
        return 0
    fi

    # try special method first
    local special
    special="$(get_install_cmd "$pkg" 2>/dev/null || true)"

    if [ -n "$special" ]; then
        # ensure go exists for go installs
        if [[ "$special" == GO111MODULE* ]] && [ -z "$GOINSTALL_BIN" ]; then
            log_message "WARN" "go não encontrado; para instalar $pkg via 'go install' instale o Go primeiro (ex: ${CMD_PACK_MANAGER_INSTALL} golang). Tentando fallback."
            special=""
        fi
    fi

    if [ -n "$special" ]; then
        log_message "INFO" "Executando método especial para $pkg: $special"
        if [ "${DRY_RUN:-0}" -eq 1 ]; then
            log_message "DRY" "(simulação) $special"
        else
            if eval "$special"; then
                log_message "SUCCESS" "Instalação especial concluída: $pkg"
                return 0
            else
                log_message "WARN" "Método especial falhou para $pkg, tentando gerenciador de pacotes..."
            fi
        fi
    fi

    # fallback: package manager
    if [ "${DRY_RUN:-0}" -eq 1 ]; then
        log_message "DRY" "(simulação) ${CMD_PACK_MANAGER_INSTALL} $pkg"
        return 0
    fi

    if _exec_install "$pkg"; then
        return 0
    else
        log_message "ERROR" "Falha ao instalar $pkg via package manager"
        return 1
    fi
}

# instala todos os itens passados como argumentos (robusto)
install_array() {
    local arr=("$@")
    local total=${#arr[@]}
    local idx=0

    for pkg in "${arr[@]}"; do
        idx=$((idx+1))
        CURRENT_PKG="$pkg"
        log_message "INFO" "[$idx/$total] Preparando a instalação de: $CURRENT_PKG"
        if ! install_one "$pkg"; then
            log_message "WARN" "Continuando após falha no pacote: $CURRENT_PKG"
        fi
        sleep "$(awk "BEGIN {print ${DELAY_MS:-300}/1000}")"
    done
}

# dedupe helper
flatten_unique() {
    declare -A seen=()
    local out=()
    for item in "$@"; do
        for v in ${item}; do
            if [ -z "${seen[$v]:-}" ]; then
                seen[$v]=1
                out+=("$v")
            fi
        done
    done
    echo "${out[@]}"
}

install_category_by_name() {
    local name="$1"
    case "$name" in
        base) install_array "${BASE_PACKAGES[@]}" ;;
        recon) install_array "${RECON_PACKAGES[@]}" ;;
        scanners) install_array "${SCANNER_PACKAGES[@]}" ;;
        web) install_array "${WEB_TOOLS[@]}" ;;
        aux) install_array "${AUX_RECON[@]}" ;;
        misc) install_array "${MISC_TOOLS[@]}" ;;
        all)
            # dedupe before installing
            local all_raw=("${BASE_PACKAGES[@]}" "${RECON_PACKAGES[@]}" "${SCANNER_PACKAGES[@]}" "${WEB_TOOLS[@]}" "${AUX_RECON[@]}" "${MISC_TOOLS[@]}")
            # call flatten_unique with single string elements -> join
            local uniq
            uniq=($(flatten_unique "${all_raw[@]}"))
            install_array "${uniq[@]}"
            ;;
        *) log_message "ERROR" "Categoria desconhecida: $name"; return 1 ;;
    esac
}

# =============================
# show_modules / verify_installed
# =============================
show_modules() {
    cat <<EOF
${BOLD}Categorias e pacotes:${NC}

1) base:
   ${BASE_PACKAGES[*]}

2) recon:
   ${RECON_PACKAGES[*]}

3) scanners:
   ${SCANNER_PACKAGES[*]}

4) web:
   ${WEB_TOOLS[*]}

5) aux:
   ${AUX_RECON[*]}

6) misc:
   ${MISC_TOOLS[*]}

Use o menu para instalar por categoria ou todas.
EOF
}

verify_installed() {
    local name="$1"
    local arr=()
    case "$name" in
        base) arr=("${BASE_PACKAGES[@]}") ;;
        recon) arr=("${RECON_PACKAGES[@]}") ;;
        scanners) arr=("${SCANNER_PACKAGES[@]}") ;;
        web) arr=("${WEB_TOOLS[@]}") ;;
        aux) arr=("${AUX_RECON[@]}") ;;
        misc) arr=("${MISC_TOOLS[@]}") ;;
        all) arr=("${BASE_PACKAGES[@]}" "${RECON_PACKAGES[@]}" "${SCANNER_PACKAGES[@]}" "${WEB_TOOLS[@]}" "${AUX_RECON[@]}" "${MISC_TOOLS[@]}") ;;
        *) log_message "ERROR" "Categoria desconhecida: $name"; return 1 ;;
    esac

    printf "\n%s\n" "Verificando instalação: $name"
    local total=${#arr[@]}
    local idx=0
    local missing=0
    for pkg in "${arr[@]}"; do
        idx=$((idx+1))
        if is_installed "$pkg"; then
            printf "%3d/%3d  %s - ${GREEN}INSTALLED${NC}\n" "$idx" "$total" "$pkg"
        else
            printf "%3d/%3d  %s - ${RED}MISSING${NC}\n" "$idx" "$total" "$pkg"
            missing=$((missing+1))
        fi
    done

    if [ "$missing" -gt 0 ]; then
        log_message "WARN" "$missing pacotes não encontrados na categoria $name"
    else
        log_message "SUCCESS" "Todos os pacotes da categoria $name parecem instalados"
    fi
}

# =============================
# Checks / boot
# =============================
verifica_basico() {
    if [ "$(id -u)" -ne 0 ] && [ -z "${SUDO_USER:-}" ]; then
        echo -e "${RED}Erro: Execute como root (sudo)!${NC}"
        exit 1
    fi

    if [ "$(id -u)" -eq 0 ] && [ -z "${SUDO_USER:-}" ]; then
        log_message "WARN" "Rodando como root direto (SUDO_USER não está definido). Isso é OK, mas preferível rodar com sudo."
    fi
}

configurar_log() {
    mkdir -p "$(dirname "$LOG_FILE")"
    # tee append all output to logfile
    exec > >(tee -a "$LOG_FILE") 2>&1
    echo -e "${BLUE}Instalação iniciada em $(date)${NC}"
}

# =============================
# Menu / UI
# =============================
print_menu() {
    echo -e "${BOLD}${GREEN}=== HyprArch Installer ===${NC}"
    echo "Escolha as categorias para instalar (pode informar múltiplas opções separadas por vírgula):"
    echo " 1) Todas"
    echo " 2) Base (utilitários: curl, jq, git, nmap)"
    echo " 3) Recon / Discovery"
    echo " 4) Scanners"
    echo " 5) Web Tools"
    echo " 6) Aux Recon"
    echo " 7) Misc / Exploitation"
    echo " 8) Mostrar módulos (listar pacotes por categoria)"
    echo " 9) Verificar instalação (escolha categoria após selecionar)"
    echo " 0) Sair"
    echo -n "Digite sua(s) opção(ões): "
}

# =============================
# Main
# =============================
main() {
    verifica_basico
    configurar_log
    detect_package_manager

    while true; do
        print_menu
        read -r input_choices
        IFS=',' read -ra choices <<< "$(echo "$input_choices" | tr -d ' ')"

        for c in "${choices[@]}"; do
            case "$c" in
                1)  log_message "INFO" "Instalando: Todas as categorias"; install_category_by_name all ;;
                2)  log_message "INFO" "Instalando: Base"; install_category_by_name base ;;
                3)  log_message "INFO" "Instalando: Recon"; install_category_by_name recon ;;
                4)  log_message "INFO" "Instalando: Scanners"; install_category_by_name scanners ;;
                5)  log_message "INFO" "Instalando: Web Tools"; install_category_by_name web ;;
                6)  log_message "INFO" "Instalando: Aux Recon"; install_category_by_name aux ;;
                7)  log_message "INFO" "Instalando: Misc"; install_category_by_name misc ;;
                8)  show_modules ;;
                9)
                    echo -n "Qual categoria verificar? (all/base/recon/scanners/web/aux/misc): "
                    read -r catcheck
                    verify_installed "$catcheck"
                    ;;
                0)  log_message "INFO" "Saindo."; exit 0 ;;
                *)  log_message "WARN" "Opção inválida: $c" ;;
            esac
        done

        echo -n "Deseja executar outra ação? (s/N): "
        read -r again
        if [[ ! "$again" =~ ^[sS] ]]; then
            break
        fi
    done

    echo -e "${GREEN}Processo finalizado. Log em: $LOG_FILE${NC}"
}

main "$@"
