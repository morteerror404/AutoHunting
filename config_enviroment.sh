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

ENV_JSON_PATH="config/json/env.json" # Caminho para o arquivo env.json
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
MENU_TYPE=""

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

show_menu_principal() {
    echo -e "\n=== Ferramenta de Configuração de Ambiente ==="
    echo -e "-----------------------------------------------\n"
    echo " 1) Utilizar configuração padrão"
    echo " 2) Mostrar configuração atual"
    echo " 3) Configurar serviço de inicialização automática"
    echo " 4) Configurar diretório para arquivos JSON"
    echo " 5) Declarar caminhos personalizados"
    echo " 6) Configuração especifica de JSON" 
    echo " 0) Voltar"
    echo
}

show_menu_personalizados() {
    echo -e "\n=== Caminhos Personalizados ==="
    echo -e "--------------------------------\n"
    echo " 1) Deseja configurar vários caminhos em sequência"
    echo " 2) Deseja configurar apenas um caminho"
    echo " 0) Voltar"
    echo

}

show_menu_joker() {
    echo -e "\n=== Configuração de $MENU_TYPE ==="
    echo -e "--------------------------------\n"
    echo " 1) Informações SUJAS"
    echo " 2) Informações LIMPAS"
    echo " 3) Templates para LIMPEZA"
    echo " 4) Comandos de FERRAMENTAS"
    echo " 5) WORDLISTs"
    echo " 6) LOGS"
    echo " 0) Voltar"
    echo
}

# ===============================
# LOOP PRINCIPAL DE NAVEGAÇÃO
# ===============================
while true; do
    show_menu_principal
    read -p "Escolha uma opção: " opcao

    case $opcao in
        1)
            echo -e "\n[+] Aplicando configuração padrão...\n"
            if ! command -v jq &> /dev/null; then
                echo -e "${RED}Erro: A ferramenta 'jq' é necessária, mas não foi encontrada. Por favor, instale-a.${NC}"
                continue
            fi

            if [ ! -f "$ENV_JSON_PATH" ]; then
                echo -e "${RED}Erro: Arquivo de configuração '$ENV_JSON_PATH' não encontrado.${NC}"
                continue
            fi

            echo "Verificando e criando diretórios definidos em '$ENV_JSON_PATH'..."
            jq -r '.path | .[]' "$ENV_JSON_PATH" | while IFS= read -r dir_path; do
                if [ -d "$dir_path" ]; then
                    echo -e "  ${GREEN}[EXISTE]${NC} O diretório '$dir_path' já existe."
                else
                    echo -e "  ${YELLOW}[CRIANDO]${NC} O diretório '$dir_path' não existe. Criando..."
                    mkdir -p "$dir_path" && echo -e "  ${GREEN}[SUCESSO]${NC} Diretório '$dir_path' criado." || echo -e "  ${RED}[FALHA]${NC}   Não foi possível criar o diretório '$dir_path'."
                fi
            done
            echo -e "\nConfiguração de diretórios concluída."
            ;;
        2)
            echo -e "\n[*] Mostrando configuração atual...\n"
            # lógica para exibir configuração atual
            ;;
        3)
            echo -e "\n[*] Configurando serviço de inicialização automática...\n"
            # lógica correspondente
            ;;
        4)
            echo -e "\n[*] Configurando diretório para arquivos JSON...\n"
            # lógica correspondente
            ;;
        5)
            while true; do
                show_menu_personalizados
                read -p "Escolha uma opção: " subopcao

                case $subopcao in
                    1)
                        echo -e "\n[*] Configurando vários caminhos em sequência...\n"
                        declare -a caminhos_desc=("Informações SUJAS" "Informações LIMPAS" "TEMPLATES para LIMPEZA" "COMANDOS de FERRAMENTAS" "WORDLISTs" "LOGS")
                        
                        for desc in "${caminhos_desc[@]}"; do
                            read -p "Caminho para: ${desc}: " user_path
                            # Aqui você adicionaria a lógica para salvar a variável 'user_path' no arquivo de configuração JSON.
                            echo "Caminho para '${desc}' definido como: ${user_path}"
                        done
                        ;;
                    2)
                        while true; do
                            show_menu_joker
                            read -p "Escolha o caminho para configurar: " caminho

                            case $caminho in
                                1) echo -e "\nConfigurando caminho: informações SUJAS\n" ;;
                                2) echo -e "\nConfigurando caminho: informações LIMPAS\n" ;;
                                3) echo -e "\nConfigurando caminho: templates para LIMPEZA\n" ;;
                                4) echo -e "\nConfigurando caminho: comandos de FERRAMENTAS\n" ;;
                                5) echo -e "\nConfigurando caminho: WORDLISTs\n" ;;
                                6) echo -e "\nConfigurando caminho: LOGS\n" ;;
                                0) break ;;
                                *) echo -e "\n[!] Opção inválida.\n" ;;
                            esac
                        done
                        ;;
                    0)
                        break
                        ;;
                    *)
                        echo -e "\n[!] Opção inválida.\n"
                        ;;
                esac
            done
            ;;
        6)
            MENU_TYPE="funcionalidade especifica"
            while true; do
                show_menu_joker
                read -p "Escolha uma opção: " joker_opcao

                case $joker_opcao in
                    1) echo -e "\nConfigurando funcionalidade: Informações SUJAS\n" ;;
                    2) echo -e "\nConfigurando funcionalidade: Informações LIMPAS\n" ;;
                    3) echo -e "\nConfigurando funcionalidade: Templates para LIMPEZA\n" ;;
                    4) echo -e "\nConfigurando funcionalidade: Comandos de FERRAMENTAS\n" ;;
                    5) echo -e "\nConfigurando funcionalidade: WORDLISTs\n" ;;
                    6) echo -e "\nConfigurando funcionalidade: LOGS\n" ;;
                    0) break ;;
                    *) echo -e "\n[!] Opção inválida.\n" ;;
                esac
            done
            ;;
        *)
            echo -e "\n[!] Opção inválida.\n"
            ;;
    esac
done

}
