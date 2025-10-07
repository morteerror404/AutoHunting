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

# ===============================
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

show_menu_archives() {
    echo -e "\n=== Configuração de Arquivos JSON ==="
    echo -e "-------------------------------------\n"
    echo " 1) Arquivo de Ordem do Maestro (maestro_exec_order)"
    echo " 2) Templates de Ordem (maestro_order_templates)"
    echo " 3) Comandos das Ferramentas (commands_json)"
    echo " 4) Tokens de API (tokens_json)"
    echo " 5) Templates do Cleaner (cleaner-templates)"
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
    echo -e "-------------------------------------------\n"
    echo " 1) Diretório de Resultados Brutos (tool_dirt_dir)"
    echo " 2) Diretório de Resultados Limpos (tool_cleaned_dir)"
    echo " 3) Diretório de Escopos Selecionados (escopos_selecionados)"
    echo " 4) Diretório de API (Resultados Brutos) (api_dirt_results_path)"
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
    echo " 3) Mostrar rotinas criadas"
    echo " 4) Excluir uma rotina" 
    echo " 0) Voltar"
    echo
}

# ===============================
# Funções de Lógica
# ===============================

update_json_value() {
    local file_path="$1"
    local key_path="$2"
    local new_value="$3"

    if ! command -v jq &> /dev/null; then
        echo -e "${RED}Erro: 'jq' não encontrado. Esta função requer jq.${NC}"
        return 1
    fi

    # Cria um arquivo temporário para a saída do jq
    local tmp_file
    tmp_file=$(mktemp)
    jq --arg key_path "$key_path" --arg new_value "$new_value" 'setpath($key_path | split("."); $new_value)' "$file_path" > "$tmp_file" && mv "$tmp_file" "$file_path"
    echo -e "${GREEN}Arquivo '$file_path' atualizado: '$key_path' definido como '$new_value'.${NC}"
}

verificar_e_criar_templates_cleaner() {
    echo -e "\n[*] Verificando e criando arquivos de template do Cleaner..."

    # 1. Obter o caminho para o arquivo principal de templates do cleaner
    local cleaner_templates_config_path
    cleaner_templates_config_path=$(jq -r '.archives."cleaner-templates"' "$ENV_JSON_PATH")

    if [ ! -f "$cleaner_templates_config_path" ]; then
        echo -e "${RED}Erro: Arquivo de configuração de templates do cleaner não encontrado em '$cleaner_templates_config_path'.${NC}"
        return 1
    fi

    # 2. Ler o arquivo e extrair todos os caminhos dos templates individuais
    jq -r '.templates | .[]' "$cleaner_templates_config_path" | while IFS= read -r template_file_path; do
        if [ -z "$template_file_path" ]; then
            continue
        fi

        local template_dir
        template_dir=$(dirname "$template_file_path")

        # 3. Verificar e criar o diretório
        if [ ! -d "$template_dir" ]; then
            echo -e "  ${YELLOW}[CRIANDO DIRETÓRIO]${NC} O diretório '$template_dir' não existe. Criando..."
            mkdir -p "$template_dir" && echo -e "  ${GREEN}[SUCESSO]${NC} Diretório '$template_dir' criado." || echo -e "  ${RED}[FALHA]${NC} Não foi possível criar o diretório '$template_dir'."
        fi

        # 4. Verificar e criar o arquivo
        if [ ! -f "$template_file_path" ]; then
            echo -e "  ${YELLOW}[CRIANDO ARQUIVO]${NC} O arquivo '$template_file_path' não existe. Criando..."
            touch "$template_file_path" && echo -e "  ${GREEN}[SUCESSO]${NC} Arquivo '$template_file_path' criado." || echo -e "  ${RED}[FALHA]${NC} Não foi possível criar o arquivo '$template_file_path'."
        fi
    done

    echo -e "\nVerificação de templates do Cleaner concluída."
}

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

    read -p "Deseja habilitar o serviço '${db_service}' para iniciar com o sistema? (s/N): " resposta
    if [[ "$resposta" =~ ^[sS]$ ]]; then
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
    else
        echo -e "[*] Operação cancelada pelo usuário."
    fi
}

criar_servico_rotina() {
    verifica_root
    local prefixo_servico="autohunt_"

    if ! command -v jq &> /dev/null || ! command -v systemctl &> /dev/null; then
        echo -e "${RED}Erro: As ferramentas 'jq' e 'systemctl' são necessárias.${NC}"
        return 1
    fi

    # --- Coleta de Informações ---
    read -p "Digite um nome para a rotina (ex: capturar_escopos_h1): " nome_servico_raw
    local nome_servico
    nome_servico=$(echo "$nome_servico_raw" | tr -cs 'a-zA-Z0-9' '_' | tr '[:upper:]' '[:lower:]')
    echo -e "[*] Nome do serviço será: ${BOLD}${nome_servico}${NC}"

    local order_templates_path
    order_templates_path=$(jq -r '.archives."maestro_order_templates"' "$ENV_JSON_PATH")
    if [ ! -f "$order_templates_path" ]; then
        echo -e "${RED}Erro: Arquivo de templates de ordem não encontrado em '$order_templates_path'. Verifique seu env.json.${NC}"
        return 1
    fi

    echo -e "\nTemplates de ordem disponíveis em '$order_templates_path':"
    local templates
    mapfile -t templates < <(jq -r 'keys[]' "$order_templates_path")
    for i in "${!templates[@]}"; do
        echo " $((i+1))) ${templates[$i]}"
    done
    read -p "Escolha o template de ordem a ser executado: " template_choice
    local template_escolhido="${templates[$((template_choice-1))]}"
    if [ -z "$template_escolhido" ]; then
        echo -e "${RED}Opção inválida.${NC}"; return 1
    fi

    local tokens_path
    tokens_path=$(jq -r '.archives."tokens_json"' "$ENV_JSON_PATH")
    echo -e "\nPlataformas disponíveis em '$tokens_path':"
    local platforms
    mapfile -t platforms < <(jq -r 'keys[]' "$tokens_path")
    for i in "${!platforms[@]}"; do
        echo " $((i+1))) ${platforms[$i]}"
    done
    read -p "Escolha a plataforma para a rotina: " platform_choice
    local plataforma_escolhida="${platforms[$((platform_choice-1))]}"
    if [ -z "$plataforma_escolhida" ]; then
        echo -e "${RED}Opção inválida.${NC}"; return 1
    fi

    echo -e "\nEscolha a frequência de execução:"
    echo " 1) Diariamente"
    echo " 2) Semanalmente"
    echo " 3) Mensalmente"
    read -p "Opção: " freq_choice

    local on_calendar=""
    local marker_format=""
    case $freq_choice in
        1) on_calendar="daily"; marker_format="%Y-%m-%d" ;;
        2) 
            read -p "Digite o dia da semana (ex: Mon, Tue, Wed, Thu, Fri, Sat, Sun): " dia_semana
            on_calendar="${dia_semana} *-*-* 02:00:00"; marker_format="%Y-%W" ;; # %W = semana do ano
        3) on_calendar="monthly"; marker_format="%Y-%m" ;;
        *) echo -e "${RED}Opção inválida.${NC}"; return 1 ;;
    esac

    # --- Criação dos Arquivos ---
    local autohunting_dir
    autohunting_dir=$(dirname "$(realpath "$0")")
    local service_markers_dir="/var/lib/autohunt/service_markers"
    mkdir -p "$service_markers_dir"

    # 1. Criar o script wrapper
    local wrapper_path="/usr/local/bin/${prefixo_servico}${nome_servico}_runner.sh"
    echo -e "[*] Criando script wrapper em '$wrapper_path'..."
    cat > "$wrapper_path" <<EOF
#!/bin/bash
set -euo pipefail

MARKER_DIR="$service_markers_dir"
MARKER_FILE="\$MARKER_DIR/${prefixo_servico}${nome_servico}_\$(date +'$marker_format').marker"

if [ -f "\$MARKER_FILE" ]; then
    echo "[\$(date)] Rotina '${nome_servico}' já executada neste período. Saindo."
    exit 0
fi

echo "[\$(date)] Iniciando rotina '${nome_servico}'..."
cd "$autohunting_dir"

# Cria o arquivo de ordem para o maestro
jq -n --arg platform "$plataforma_escolhida" --arg task "$template_escolhido" \
  '.platform = \$platform | .task = \$task' > order.json

# Executa o maestro (assumindo que o binário está na raiz do projeto)
if ./maestro; then
    echo "[\$(date)] Rotina '${nome_servico}' executada com sucesso."
    touch "\$MARKER_FILE"
else
    echo "[\$(date)] Erro ao executar a rotina '${nome_servico}'."
    exit 1
fi
EOF
    chmod +x "$wrapper_path"

    # 2. Criar o arquivo .service
    local service_path="/etc/systemd/system/${prefixo_servico}${nome_servico}.service"
    echo -e "[*] Criando arquivo de serviço em '$service_path'..."
    cat > "$service_path" <<EOF
[Unit]
Description=[AutoHunting] Serviço para a rotina '${nome_servico_raw}'

[Service]
Type=oneshot
ExecStart=$wrapper_path
User=$(logname) # Executa como o usuário que configurou
EOF

    # 3. Criar o arquivo .timer
    local timer_path="/etc/systemd/system/${prefixo_servico}${nome_servico}.timer"
    echo -e "[*] Criando arquivo de timer em '$timer_path'..."
    cat > "$timer_path" <<EOF
[Unit]
Description=[AutoHunting] Timer para a rotina '${nome_servico_raw}'

[Timer]
OnCalendar=$on_calendar
Persistent=true

[Install]
WantedBy=timers.target
EOF

    # --- Ativação ---
    echo -e "[*] Recarregando, habilitando e iniciando o timer..."
    systemctl daemon-reload
    systemctl enable "${prefixo_servico}${nome_servico}.timer"
    systemctl start "${prefixo_servico}${nome_servico}.timer"

    echo -e "${GREEN}Serviço e timer '${prefixo_servico}${nome_servico}' configurados com sucesso!${NC}"
    systemctl list-timers --all | grep "${prefixo_servico}${nome_servico}"
}

mostrar_rotinas_criadas() {
    echo -e "\n[*] Verificando rotinas do AutoHunting criadas...\n"
    if ! systemctl list-timers --all | grep -q '\[AutoHunting\]'; then
        echo -e "${YELLOW}Nenhuma rotina agendada do AutoHunting foi encontrada.${NC}"
        return
    fi
    
    echo -e "${BOLD}Rotinas agendadas:${NC}"
    systemctl list-timers --all | grep '\[AutoHunting\]' --color=never
}

excluir_rotina() {
    verifica_root
    local prefixo_servico="autohunt_"

    echo -e "\n[*] Listando rotinas que podem ser excluídas..."
    local rotinas
    mapfile -t rotinas < <(ls -1 /etc/systemd/system/${prefixo_servico}*.timer 2>/dev/null | xargs -n 1 basename | sed "s/${prefixo_servico}//" | sed 's/\.timer//')

    if [ ${#rotinas[@]} -eq 0 ]; then
        echo -e "${YELLOW}Nenhuma rotina criada pelo AutoHunting para excluir.${NC}"
        return
    fi

    for i in "${!rotinas[@]}"; do
        echo " $((i+1))) ${rotinas[$i]}"
    done
    echo " 0) Cancelar"

    read -p "Escolha a rotina para excluir: " choice
    if ! [[ "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 0 ] || [ "$choice" -gt ${#rotinas[@]} ]; then
        echo -e "${RED}Opção inválida.${NC}"; return 1
    fi
    if [ "$choice" -eq 0 ]; then
        echo "[*] Operação cancelada."; return
    fi

    local nome_rotina="${rotinas[$((choice-1))]}"
    local nome_completo="${prefixo_servico}${nome_rotina}"

    read -p "Tem certeza que deseja excluir permanentemente a rotina '${nome_rotina}'? (s/N): " confirm
    if [[ ! "$confirm" =~ ^[sS]$ ]]; then
        echo "[*] Exclusão cancelada."; return
    fi

    echo -e "[*] Desabilitando e parando o timer '${nome_completo}.timer'..."
    systemctl disable --now "${nome_completo}.timer"

    echo -e "[*] Removendo arquivos de serviço e timer..."
    rm -f "/etc/systemd/system/${nome_completo}.service"
    rm -f "/etc/systemd/system/${nome_completo}.timer"

    echo -e "[*] Removendo script wrapper..."
    rm -f "/usr/local/bin/${nome_completo}_runner.sh"

    echo -e "[*] Recarregando o systemd..."
    systemctl daemon-reload

    echo -e "${GREEN}Rotina '${nome_rotina}' excluída com sucesso.${NC}"
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
                        criar_servico_rotina
                        ;;
                    3)
                        mostrar_rotinas_criadas
                        ;;
                    4)
                        echo -e "\n[*] Excluir uma rotina de usuário...\n"
                        excluir_rotina
                        ;;
                    0) break ;;
                    *) echo -e "\n[!] Opção inválida.\n" ;;
                esac
            done
            ;;
        4)
            while true; do
                show_menu_archives
                read -p "Escolha o arquivo para configurar: " archive_choice

                local key_to_update=""
                case $archive_choice in
                    1) key_to_update="maestro_exec_order" ;;
                    2) key_to_update="maestro_order_templates" ;;
                    3) key_to_update="commands_json" ;;
                    4) key_to_update="tokens_json" ;;
                    5) 
                        verificar_e_criar_templates_cleaner
                        key_to_update="cleaner-templates" 
                        ;;
                    0) break ;;
                    *) echo -e "\n[!] Opção inválida.\n"; continue ;;
                esac

                read -p "Digite o novo caminho para '$key_to_update': " new_path
                if [ -n "$new_path" ]; then
                    # O caminho da chave no JSON é '.archives.nome_da_chave'
                    update_json_value "$ENV_JSON_PATH" ".archives.$key_to_update" "$new_path"
                else
                    echo -e "${YELLOW}Nenhum caminho fornecido. Operação cancelada.${NC}"
                fi
            done
            ;;
        5)
            while true; do
                show_menu_personalizados
                read -p "Escolha uma opção: " subopcao

                case $subopcao in
                    1)
                        echo -e "\n[*] Configurando vários caminhos em sequência...\n"
                        # Mapeia a descrição para a chave real no JSON
                        declare -A path_map=(
                            ["Resultados Brutos das Ferramentas"]="tool_dirt_dir"
                            ["Resultados Limpos das Ferramentas"]="tool_cleaned_dir"
                            ["Resultados Brutos da API"]="api_dirt_results_path"
                            ["Resultados Limpos da API"]="api_clean_results_path"
                            ["Diretório de Wordlists"]="wordlist_dir"
                            ["Diretório de Logs"]="log_dir"
                        )
                        
                        for desc in "${!path_map[@]}"; do
                            read -p "Caminho para: ${desc}: " new_path
                            if [ -n "$new_path" ]; then
                                local key="${path_map[$desc]}"
                                update_json_value "$ENV_JSON_PATH" ".path.$key" "$new_path"
                            fi
                        done
                        ;;
                    2)
                        MENU_TYPE="Caminho"
                        while true; do
                            show_menu_joker
                            read -p "Escolha o caminho para configurar: " caminho

                            local key_to_update=""
                            case $caminho in
                                1) key_to_update="tool_dirt_dir" ;;
                                2) key_to_update="tool_cleaned_dir" ;;
                                3) key_to_update="escopos_selecionados" ;;
                                4) key_to_update="api_dirt_results_path" ;;
                                5) key_to_update="wordlist_dir" ;;
                                6) key_to_update="log_dir" ;;
                                0) break ;;
                                *) echo -e "\n[!] Opção inválida.\n"; continue ;;
                            esac

                            read -p "Digite o novo caminho para '$key_to_update': " new_path
                            if [ -n "$new_path" ]; then
                                update_json_value "$ENV_JSON_PATH" ".path.$key_to_update" "$new_path"
                            else
                                echo -e "${YELLOW}Nenhum caminho fornecido. Operação cancelada.${NC}"
                            fi
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
            MENU_TYPE="funcionalidade específica"
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
