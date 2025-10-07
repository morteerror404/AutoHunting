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
log() {
    # Função de log simples, pode ser expandida se necessário.
    echo "[$1] $2"
}

verifica_root() {
    if [ "$(id -u)" -ne 0 ] && [ -z "${SUDO_USER:-}" ]; then
        echo -e "${RED}Erro: Execute como root (sudo)!${NC}"
        exit 1
    fi
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

show_menu_servicos() {
    echo -e "\n=== Configuração de Serviços ==="
    echo -e "--------------------------------\n"
    echo " 1) Serviço para inicializar o banco de dados"
    echo " 2) Criar o serviço para iniciar uma rotina especifica"
    echo " 0) Voltar"
    echo
}

# ===============================
# Funções de Lógica
# ===============================

configurar_servico_db() {
    verifica_root

    if ! command -v systemctl &> /dev/null; then
        echo -e "${RED}Erro: O comando 'systemctl' não foi encontrado. Este recurso está disponível apenas em sistemas com systemd.${NC}"
        return 1
    fi

    local marker_dir="/var/lib/autohunt/markers"
    local db_service=""

    # Detecta o serviço do banco de dados com base nos arquivos de marcador
    if ls "$marker_dir"/bughunt.postgresql.marker* 1> /dev/null 2>&1; then
        db_service="postgresql"
    elif ls "$marker_dir"/bughunt.mariadb.marker* 1> /dev/null 2>&1; then
        db_service="mariadb"
    elif ls "$marker_dir"/bughunt.mysql.marker* 1> /dev/null 2>&1; then
        db_service="mysql"
    elif ls "$marker_dir"/bughunt.mongodb.marker* 1> /dev/null 2>&1; then
        db_service="mongod" # O nome do serviço geralmente é 'mongod'
    else
        echo -e "${YELLOW}Aviso: Nenhum banco de dados configurado pelo 'db_config.sh' foi detectado.${NC}"
        echo -e "Por favor, execute o script 'db_config.sh' primeiro."
        return 1
    fi

    echo -e "[*] Banco de dados detectado: ${BOLD}${db_service}${NC}"

    echo -e "[*] Habilitando o serviço '${db_service}' para iniciar com o sistema..."
    if systemctl enable "${db_service}.service"; then
        echo -e "${GREEN}Serviço habilitado com sucesso.${NC}"
    else
        echo -e "${RED}Falha ao habilitar o serviço.${NC}"
    fi

    echo -e "[*] Iniciando o serviço '${db_service}'..."
    if systemctl start "${db_service}.service"; then
        echo -e "${GREEN}Serviço iniciado com sucesso.${NC}"
    else
        echo -e "${RED}Falha ao iniciar o serviço.${NC}"
    fi
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
            if ! command -v jq &> /dev/null; then
                echo -e "${RED}Erro: A ferramenta 'jq' é necessária, mas não foi encontrada. Por favor, instale-a.${NC}"
                continue
            fi

            if [ ! -f "$ENV_JSON_PATH" ]; then
                echo -e "${RED}Erro: Arquivo de configuração '$ENV_JSON_PATH' não encontrado.${NC}"
                continue
            fi

            echo -e "${BOLD}Caminhos de Diretórios:${NC}"
            jq -r '.path | to_entries[] | "\(.key):\(.value)"' "$ENV_JSON_PATH" | while IFS=: read -r key value; do
                printf "  ${GREEN}%-30s${NC} %s\n" "$key" "$value"
            done

            echo -e "\n${BOLD}Arquivos de Configuração:${NC}"
            jq -r '.archives | to_entries[] | "\(.key):\(.value)"' "$ENV_JSON_PATH" | while IFS=: read -r key value; do
                printf "  ${CYAN}%-30s${NC} %s\n" "$key" "$value"
            done
            echo
            ;;
        3)
            while true; do
                show_menu_servicos
                read -p "Escolha uma opção: " servico_opcao

                case $servico_opcao in
                    1)
                        echo -e "\n[*] Configurando serviço para o banco de dados...\n"
                        configurar_servico_db
                        ;;
                    2)
                        echo -e "\n[*] Configurando serviço para rotina específica...\n"
                        # Lógica para configurar a rotina
                        ;;
                    0) break ;;
                    *) echo -e "\n[!] Opção inválida.\n" ;;
                esac
            done
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
