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

# Mapear pacotes para repositórios Git
declare -A GIT_REPOS=(
    [curl]="https://github.com/curl/curl"
    [jq]="https://github.com/jqlang/jq"
    [git]="https://github.com/git/git"
    [go]="https://github.com/golang/go"
    [subfinder]="https://github.com/projectdiscovery/subfinder"
    [amass]="https://github.com/OWASP/Amass"
    [assetfinder]="https://github.com/tomnomnom/assetfinder"
    [katana]="https://github.com/projectdiscovery/katana"
    [httpx]="https://github.com/projectdiscovery/httpx"
    [gau]="https://github.com/lc/gau"
    [waybackurls]="https://github.com/tomnomnom/waybackurls"
    [awsbucketdump]="https://github.com/jordanpotti/AWSBucketDump"
    [aquatone]="https://github.com/michenriksen/aquatone"
    [eyewitness]="https://github.com/RedSiege/EyeWitness"
    [linkfinder]="https://github.com/GerbenJavado/LinkFinder"
    [gf]="https://github.com/tomnomnom/gf"
    [gitdumper]="https://github.com/arthaud/gitdumper"
    [reconftw]="https://github.com/six2dez/reconftw"
    [bugbountytoolkit]="https://github.com/hackerguyarjun/bugbountytoolkit"
    [nmap]="https://github.com/nmap/nmap"
    [nuclei]="https://github.com/projectdiscovery/nuclei"
    [ffuf]="https://github.com/ffuf/ffuf"
    [gobuster]="https://github.com/OJ/gobuster"
    [nikto]="https://github.com/sullo/nikto"
    [sqlmap]="https://github.com/sqlmapproject/sqlmap"
    [wafw00f]="https://github.com/EnableSecurity/wafw00f"
    [dalfox]="https://github.com/hahwul/dalfox"
    [xsstrike]="https://github.com/s0md3v/XSStrike"
    [gopherus]="https://github.com/tarunkant/Gopherus"
    [lfisuite]="https://github.com/D35m0nd142/LFISuite"
    [fimap]="https://github.com/kurobeats/fimap"
    [oralyzer]="https://github.com/r0eXpeR/Oralyzer"
    [cmseek]="https://github.com/Tuhinshubhra/CMSeeK"
    [kiterunner]="https://github.com/assetnote/kiterunner"
    [metasploit]="https://github.com/rapid7/metasploit-framework"
)

# =============================
# Pacotes de ferramentas
# =============================
BASE_PACKAGES=(curl jq git nmap go python)
RECON_PACKAGES=(subfinder amass assetfinder katana httpx gau waybackurls reconftw bugbountytoolkit awsbucketdump)
SCANNER_PACKAGES=(nuclei ffuf gobuster nikto sqlmap wafw00f dalfox xsstrike)
WEB_TOOLS=(gopherus lfisuite fimap oralyzer cmseek)
AUX_RECON=(ffuf kiterunner gau waybackurls awsbucketdump aquatone eyewitness)
MISC_TOOLS=(linkfinder gf gitdumper metasploit)

# =============================
# Helpers e logging
# =============================
log_message() {
    local level="$1"; shift
    local msg="$*"
    echo -e "[$(date '+%F %T')] ${level}: $msg"
}

# =============================
# Detect & package manager
# =============================
detect_package_manager() {
    local prefix=""
    if [ "$(id -u)" -ne 0 ]; then
        prefix="sudo "
    fi

    if command -v apt >/dev/null 2>&1; then
        CMD_PACK_MANAGER_INSTALL="${prefix}apt install -y"
        CMD_PACK_MANAGER_NAME="apt"
    elif command -v pacman >/dev/null 2>&1; then
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
    log_message "SUCCESS" "Gerenciador de pacotes detectado: $CMD_PACK_MANAGER_NAME"
}

# =============================
# Exec install genérico
# =============================
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
# Map package -> install method
# =============================

# Função caso de uma merda com o gerenciador de pacotes e no git 
get_install_cmd() {
    local pkg="$1"
    case "$pkg" in
        # instala direto pelo Go
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
        # instala com o pip do python
        linkfinder) echo "pip3 install linkfinder || true" ;;
        wafw00f) echo "pip3 install wafw00f || true" ;;
        xsstrike) echo "pip3 install xsstrike || true" ;;
        # instala com git
        katana) echo "git clone https://github.com/projectdiscovery/katana.git ${TOOLS_PREFIX}/katana || true" ;;
        reconftw) echo "git clone https://github.com/six2dez/reconftw.git ${TOOLS_PREFIX}/reconftw || true" ;;
        eyewitness) echo "git clone https://github.com/FortyNorthSecurity/EyeWitness.git ${TOOLS_PREFIX}/EyeWitness || true" ;;
        nuclei-templates) echo "git clone https://github.com/projectdiscovery/nuclei-templates.git ${TOOLS_PREFIX}/nuclei-templates || true" ;;
        *) echo "" ;;
    esac
}

# =============================
# Package binary candidates
# =============================
pkg_binary_candidates() {
    local pkg="$1"
    case "$pkg" in
        metasploit) echo "msfconsole" ;;
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
# is_installed
# =============================

# verifica se já está instalado 
is_installed() {
    local pkg="$1"
    local candidates
    candidates="$(pkg_binary_candidates "$pkg")"
    for c in $candidates; do
        if command -v "$c" >/dev/null 2>&1; then
            return 0
        fi
    done

    local lower="${pkg,,}"
    for c in "$pkg" "${lower}" "${pkg//_/-}" "${pkg//-/_}"; do
        if command -v "$c" >/dev/null 2>&1; then
            return 0
        fi
    done

    case "${CMD_PACK_MANAGER_NAME-}" in
        apt) dpkg -s "$pkg" >/dev/null 2>&1 && return 0 ;;
        pacman) pacman -Qi "$pkg" >/dev/null 2>&1 && return 0 ;;
        dnf|yum) rpm -q "$pkg" >/dev/null 2>&1 && return 0 ;;
    esac
    return 1
}

# =============================
# Install one package
# =============================

# instala só um pacote, Não sei porque o usuário vai escolher essa opção mais tem
install_one() {
    local pkg="$1"

    # já instalado
    if is_installed "$pkg"; then
        log_message "INFO" "Pulando (já instalado): $pkg"
        TOOLS_PM_INSTALLED+=("$pkg")
        return 0
    fi

    # método especial
    local special
    special="$(get_install_cmd "$pkg" 2>/dev/null || true)"

    if [[ -n "$special" ]]; then
        if [[ "$special" == GO111MODULE* ]] && [[ -z "$GOINSTALL_BIN" ]]; then
            log_message "WARN" "Go não encontrado; instalando $pkg via package manager..."
            special=""
        fi
    fi

    if [[ -n "$special" ]]; then
        log_message "INFO" "Executando método especial: $special"
        if [ "${DRY_RUN:-0}" -eq 1 ]; then
            log_message "DRY" "(simulação) $special"
        else
            eval "$special" && {
                log_message "SUCCESS" "Instalado via método especial: $pkg"
                return 0
            } || log_message "WARN" "Falhou método especial: $pkg"
        fi
    fi

    # fallback PM
    if _exec_install "$pkg"; then
        TOOLS_PM_INSTALLED+=("$pkg")
        return 0
    fi

    # fallback Git
    local git_url="${GIT_REPOS[$pkg]:-}"
    if [[ -z "$git_url" ]]; then
        log_message "WARN" "Nenhum GitHub configurado para $pkg"
        return 1
    fi

    if ping -c1 -W2 "$(echo "$git_url" | awk -F/ '{print $3}')" >/dev/null 2>&1; then
        if [[ -z "$GIT_INSTALL_DIR" || "$GIT_INSTALL_DIR" == "False" ]]; then
            read -rp "Diretório Git (enter = pwd, n/no = pwd): " user_dir
            [[ -z "$user_dir" || "$user_dir" =~ ^(n|no)$ ]] && GIT_INSTALL_DIR="$(pwd)" || GIT_INSTALL_DIR="$user_dir"
        fi

        local install_dir="$GIT_INSTALL_DIR/$pkg"
        mkdir -p "$install_dir"
        git clone "$git_url" "$install_dir" || { log_message "ERROR" "Falha ao clonar $pkg"; return 1; }

        TOOLS_GIT_INSTALLED+=("$pkg")
        echo "URL = $git_url" > "$install_dir/install_info.txt"
        echo "Dir = $install_dir" >> "$install_dir/install_info.txt"
        log_message "SUCCESS" "Instalado via Git: $pkg"
    else
        log_message "ERROR" "Não foi possível conectar ao GitHub para $pkg"
    fi
}

# =============================
# Install array with parallel
# =============================

# usa paralelismo para deixar o script mais rapido 

install_array() {
    local arr=("$@")
    local idx=0 total=${#arr[@]}
    local jobs=0
    for pkg in "${arr[@]}"; do
        idx=$((idx+1))
        log_message "INFO" "[$idx/$total] Instalando: $pkg"
        install_one "$pkg" &
        jobs=$((jobs+1))
        if [ "$jobs" -ge "$MAX_JOBS" ]; then
            wait
            jobs=0
        fi
    done
    wait
}

# =============================
# Flatten unique
# =============================
flatten_unique() {
    declare -A seen=()
    local out=()
    for item in "$@"; do
        [[ -z "${seen[$item]:-}" ]] && seen[$item]=1 && out+=("$item")
    done
    echo "${out[@]}"
}

# =============================
# Category install
# =============================
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
            local all_raw=("${BASE_PACKAGES[@]}" "${RECON_PACKAGES[@]}" "${SCANNER_PACKAGES[@]}" \
                "${WEB_TOOLS[@]}" "${AUX_RECON[@]}" "${MISC_TOOLS[@]}")
            local uniq
            uniq=($(flatten_unique "${all_raw[@]}"))
            install_array "${uniq[@]}"
            ;;
        *) log_message "ERROR" "Categoria desconhecida: $name"; return 1 ;;
    esac
}

# =============================
# Show modules
# =============================
show_modules() {
    cat <<EOF
${BOLD}Categorias e pacotes:${NC}

1) base: ${BASE_PACKAGES[*]}
2) recon: ${RECON_PACKAGES[*]}
3) scanners: ${SCANNER_PACKAGES[*]}
4) web: ${WEB_TOOLS[*]}
5) aux: ${AUX_RECON[*]}
6) misc: ${MISC_TOOLS[*]}

EOF
}

# =============================
# Show install summary
# =============================
show_install_summary() {
    echo -e "\n${BOLD}${CYAN}=== Resumo das instalações ===${NC}"
    echo -e "${BOLD}Tools installed by $CMD_PACK_MANAGER_NAME:${NC}"
    for t in "${TOOLS_PM_INSTALLED[@]}"; do echo "  $t"; done
    echo -e "${BOLD}Tools installed by git clone:${NC}"
    for t in "${TOOLS_GIT_INSTALLED[@]}"; do
        local info_dir="$GIT_INSTALL_DIR/$t/install_info.txt"
        echo "  $t"
        [[ -f "$info_dir" ]] && cat "$info_dir" | sed 's/^/    /'
    done
}

# =============================
# Boot checks
# =============================
verifica_basico() {
    if [ "$(id -u)" -ne 0 ] && [ -z "${SUDO_USER:-}" ]; then
        echo -e "${RED}Erro: Execute como root (sudo)!${NC}"; exit 1
    fi
}

configurar_log() {
    mkdir -p "$(dirname "$LOG_FILE")"
    exec > >(tee -a "$LOG_FILE") 2>&1
    echo -e "${BLUE}Instalação iniciada em $(date)${NC}"
}

# =============================
# Menu
# =============================
print_menu() {
    echo -e "${BOLD}${GREEN}=== Installer ===${NC}"
    echo "Escolha categorias (múltiplas separadas por vírgula):"
    echo " 1) Todas"
    echo " 2) Base"
    echo " 3) Recon"
    echo " 4) Scanners"
    echo " 5) Web"
    echo " 6) Aux"
    echo " 7) Misc"
    echo " 8) Mostrar módulos"
    echo " 9) Verificar instalação"
    echo " 0) Sair"
    echo -n "Opção(s): "
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
        read -r options
        # Handle multiple comma-separated options
        IFS=',' read -ra opts <<< "$options"
        for opt in "${opts[@]}"; do
            case "$opt" in
                1) install_category_by_name "all" ;;
                2) install_category_by_name "base" ;;
                3) install_category_by_name "recon" ;;
                4) install_category_by_name "scanners" ;;
                5) install_category_by_name "web" ;;
                6) install_category_by_name "aux" ;;
                7) install_category_by_name "misc" ;;
                8) show_modules ;;
                9)
                    echo -e "${BOLD}Verificando instalações...${NC}"
                    for pkg in "${BASE_PACKAGES[@]}" "${RECON_PACKAGES[@]}" "${SCANNER_PACKAGES[@]}" "${WEB_TOOLS[@]}" "${AUX_RECON[@]}" "${MISC_TOOLS[@]}"; do
                        if is_installed "$pkg"; then
                            log_message "INFO" "$pkg: Instalado"
                        else
                            log_message "WARN" "$pkg: Não instalado"
                        fi
                    done
                    ;;
                0)
                    show_install_summary
                    echo -e "${BOLD}${GREEN}Instalação concluída!${NC}"
                    exit 0
                    ;;
                *)
                    log_message "ERROR" "Opção inválida: $opt"
                    ;;
            esac
        done
    done
}
main