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
RETENTION_SCRIPT="/usr/local/bin/autohunt_retention_cleanup.sh"
CRON_MARKER="/etc/cron.d/autohunt_retention"
RETENTION_DAYS=30

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
    echo " 1) Verificar/Recriar diretórios base (executado no install.sh)"
    echo " 2) Mostrar configuração atual"
    echo " 3) Configurar serviço de inicialização automática"
    echo " 4) Configurar diretório para arquivos JSON"
    echo " 5) Declarar caminhos personalizados"
    echo " 6) Configuração especifica de JSON"
    echo " 7) Imunizar/Desimunizar um banco de dados"
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
    echo " 5) Diretório de WORDLISTs"
    echo " 6) Diretório de LOGS"
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
    echo " 5) Gerenciar política de retenção de dados (Cron)"
    echo " 0) Voltar"
    echo
}

show_menu_Wordlist() {
    echo -e "\n=== Caminhos para wordlist ==="
    echo -e "--------------------------------\n"
    echo " 1) Deseja configurar manualmente "
    echo " 2) Mapear cada wordlists automaticamente"
    echo " 0) Voltar"
    echo
}

show_menu_retention() {
    echo -e "\n=== Política de Retenção de Dados ==="
    echo -e "-------------------------------------\n"
    echo " 1) Ativar e configurar cron de retenção"
    echo " 2) Desativar cron de retenção"
    echo " 3) Definir dias de retenção (atual: $RETENTION_DAYS)"
    echo " 0) Voltar"
    echo
}

show_menu_logs() {
    echo -e "\n=== Configuração de Logs ==="
    echo -e "--------------------------------\n"
    echo " 1) Definir diretório de logs"
    echo " 2) Configurar tipo de saída do log (Arquivo/Terminal)"
    echo " 3) Configurar política de retenção de logs"
    echo " 0) Voltar"
    echo
}

show_menu_logs() {
    echo -e "\n=== Configuração de Logs ==="
    echo -e "--------------------------------\n"
    echo " 1) Definir diretório de logs"
    echo " 2) Configurar tipo de saída do log (Arquivo/Terminal)"
    echo " 3) Configurar política de retenção de logs"
    echo " 0) Voltar"
    echo
}

immunize_item() {
    local name="$1"
    local dbtype="$2"
    local immun_file="/var/lib/autohunt/markers/${name}.${dbtype}.immunized"
    
    if [ -f "$immun_file" ]; then
        read -p "Este item já está imunizado. Deseja remover a imunização? (s/N): " confirm
        if [[ "$confirm" =~ ^[sS]$ ]]; then
            rm -f "$immun_file"
            echo -e "${GREEN}Imunização de '$name' ($dbtype) removida.${NC}"
        fi
    else
        touch "$immun_file"
        chmod 600 "$immun_file"
        echo -e "${GREEN}Item '$name' ($dbtype) foi imunizado contra a política de retenção.${NC}"
    fi
}

handle_immunize_menu() {
    local marker_dir="/var/lib/autohunt/markers"
    
    echo -e "\n[*] Listando bancos de dados com marcadores de retenção..."
    
    local markers=()
    while IFS= read -r marker_file; do
        markers+=("$marker_file")
    done < <(find "$marker_dir" -name "*.marker")

    if [ ${#markers[@]} -eq 0 ]; then
        echo -e "${YELLOW}Nenhum banco de dados com marcador de retenção encontrado.${NC}"
        return
    fi

    for i in "${!markers[@]}"; do
        local base_name=$(basename "${markers[$i]}" .marker)
        local db_name=${base_name%%.*}
        local db_type=${base_name#*.}
        
        local status=""
        if [ -f "$marker_dir/${db_name}.${db_type}.immunized" ]; then
            status="${GREEN}(Imunizado)${NC}"
        else
            status="${YELLOW}(Não Imunizado)${NC}"
        fi
        echo " $((i+1))) $db_name ($db_type) $status"
    done
    
    read -p "Escolha o número do banco de dados para imunizar/desimunizar (ou 0 para sair): " choice
    
    if [[ "$choice" -gt 0 && "$choice" -le ${#markers[@]} ]]; then
        local selected_marker=${markers[$((choice-1))]}
        local base_name=$(basename "$selected_marker" .marker)
        local db_name=${base_name%%.*}
        local db_type=${base_name#*.}
        immunize_item "$db_name" "$db_type"
    else
        echo "Seleção inválida ou cancelada."
    fi
}

install_retention_script() {
    log "INFO" "Escrevendo script de retenção em $RETENTION_SCRIPT"
    cat > "$RETENTION_SCRIPT" <<'EOF'
#!/usr/bin/env bash
# autohunt_retention_cleanup.sh
MARKER_DIR="/var/lib/autohunt/markers"
RETENTION_DAYS_DEFAULT=30
LOGFILE="/var/log/autohunting_install.log" # Ajustado para o log do ambiente
log() { printf '[%s] [%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$1" "$2" | tee -a "$LOGFILE"; }
RETENTION_DAYS="${RETENTION_DAYS:-$RETENTION_DAYS_DEFAULT}"
for marker in "$MARKER_DIR"/*.marker 2>/dev/null; do
    [ -f "$marker" ] || continue
    stamp=$(cat "$marker" 2>/dev/null || echo 0)
    if ! [[ "$stamp" =~ ^[0-9]+$ ]]; then
        log "WARN" "Marker $marker invalid timestamp"
        continue
    fi
    age_days=$(( ( $(date +%s) - stamp ) / 86400 ))
    base=$(basename "$marker")
    name="${base%%.*}"
    dbtype="${base#*.}"
    dbtype="${dbtype%.marker}"
    if [ -f "$MARKER_DIR/${name}.${dbtype}.immunized" ]; then
        log "DEBUG" "Marker $marker is immunized, skipping."
        continue
    fi
    if [ "$age_days" -ge "$RETENTION_DAYS" ]; then
        log "INFO" "Marker $marker age $age_days >= $RETENTION_DAYS: removing database $name for $dbtype"
        case "$dbtype" in
            postgresql)
                if command -v psql >/dev/null 2>&1; then
                    sudo -u postgres psql -c "DROP DATABASE IF EXISTS ${name};" 2>&1 | tee -a "$LOGFILE" || log "ERROR" "Failed drop ${name} on postgres"
                else
                    log "ERROR" "psql não encontrado."
                fi
                ;;
            mariadb|mysql)
                if command -v mysql >/dev/null 2>&1; then
                    mysql -u root -e "DROP DATABASE IF EXISTS \`${name}\`;" 2>&1 | tee -a "$LOGFILE" || log "ERROR" "Failed drop ${name} on mysql/mariadb"
                else
                    log "ERROR" "mysql client não encontrado."
                fi
                ;;
            mongodb)
                local mongo_cmd
                if command -v mongosh >/dev/null 2>&1; then
                    mongo_cmd="mongosh"
                elif command -v mongo >/dev/null 2>&1; then
                    mongo_cmd="mongo"
                else
                    log "ERROR" "Nem mongosh nem mongo encontrados."
                    continue
                fi
                $mongo_cmd --quiet --eval "db.getSiblingDB('${name}').dropDatabase()" 2>&1 | tee -a "$LOGFILE" || log "ERROR" "Failed drop ${name} on mongodb"
                ;;
            *)
                log "WARN" "Unknown db type: $dbtype for marker $marker"
                ;;
        esac
        mv "$marker" "${marker}.removed.$(date +%s)" 2>&1 | tee -a "$LOGFILE" || rm -f "$marker" 2>&1 | tee -a "$LOGFILE"
        log "INFO" "Database ${name} removal attempted and marker archived/removed."
    else
        log "DEBUG" "Marker $marker age $age_days (<$RETENTION_DAYS) -> keep"
    fi
done
EOF
    chmod 750 "$RETENTION_SCRIPT"
    log "INFO" "Script de retenção escrito e autorizado em $RETENTION_SCRIPT"
}

install_terminal_warning() {
    local warnfile="/etc/profile.d/autohunt_retention_warning.sh"
    cat > "$warnfile" <<'EOF'
#!/usr/bin/env bash
MARKER_DIR="/var/lib/autohunt/markers"
MANAGER="config_enviroment.sh" # Aponta para o script correto
for m in "$MARKER_DIR"/*.marker 2>/dev/null; do
    [ -f "$m" ] || continue
    name="$(basename "$m")"
    base="${name%.marker}"
    db="$(echo "$base" | awk -F. '{print $2}')"
    item="$(echo "$base" | awk -F. '{print $1}')"
    stamp=$(cat "$m" 2>/dev/null || echo 0)
    if ! [[ "$stamp" =~ ^[0-9]+$ ]]; then continue; fi
    RETENTION_DAYS=${RETENTION_DAYS:-30}
    expiry=$((stamp + (RETENTION_DAYS * 86400)))
    now=$(date +%s)
    if [ "$expiry" -gt "$now" ]; then
        seconds_left=$((expiry - now))
        days_left=$((seconds_left / 86400))
        hours_left=$(((seconds_left % 86400) / 3600))
        echo -e "\033[1;33mAVISO AUTOhunt: O banco de dados '\033[1;31m$item\033[0;33m' (tipo: \033[1;31m$db\033[0;33m) será excluído em \033[1;31m${days_left}\033[0;33m dias e \033[1;31m${hours_left}\033[0;33m horas\033[0m"
        echo -e "Para imunizar, execute o script de configuração de ambiente e escolha a opção de imunização."
        echo
    fi
done
EOF
    chmod 644 "$warnfile"
    log "INFO" "Aviso de retenção instalado em $warnfile."
}

remove_terminal_warning() {
    local warnfile="/etc/profile.d/autohunt_retention_warning.sh"
    if [ -f "$warnfile" ]; then
        rm -f "$warnfile"
        log "INFO" "Aviso de retenção removido ($warnfile)."
    fi
}

enable_retention_cron() {
    verifica_root
    install_retention_script

    echo -e "${BLUE}Configurando cron job de retenção:${NC}"
    read -p "Horário do cron (HH:MM, padrão 03:30): " cron_time
    cron_time="${cron_time:-03:30}"
    read -p "Frequência (1-Diária, 2-Semanal, 3-Mensal, padrão 1): " freq_opt

    local cron_min="${cron_time#*:}"
    local cron_hour="${cron_time%:*}"
    local cron_daymonth="*" cron_month="*" cron_dayweek="*"
    case "$freq_opt" in
        2) cron_dayweek="0" ;; # Domingo
        3) cron_daymonth="1" ;; # Dia 1 do mês
    esac

    echo "$cron_min $cron_hour $cron_daymonth $cron_month $cron_dayweek root $RETENTION_SCRIPT" > "$CRON_MARKER"
    chmod 644 "$CRON_MARKER"
    log "SUCCESS" "Cron de retenção ativado: $CRON_MARKER"
    install_terminal_warning
}

disable_retention_cron() {
    verifica_root
    rm -f "$CRON_MARKER"
    log "INFO" "Cron de retenção desativado."
    remove_terminal_warning
}

# ===============================
# Funções de Lógica
# ===============================

verificar_e_criar_diretorios_base() {
    log "INFO" "Verificando e criando diretórios base definidos em '$ENV_JSON_PATH'..."
    if ! command -v jq &> /dev/null; then
        log "ERROR" "A ferramenta 'jq' é necessária para ler as configurações, mas não foi encontrada."
        return 1
    fi

    if [ ! -f "$ENV_JSON_PATH" ]; then
        log "ERROR" "Arquivo de configuração '$ENV_JSON_PATH' não encontrado. Não é possível criar os diretórios."
        return 1
    fi

    # Itera sobre todos os valores dentro do objeto 'path'
    jq -r '.path | to_entries[] | .value' "$ENV_JSON_PATH" | while IFS= read -r dir_path; do
        if [ -z "$dir_path" ] || [ "$dir_path" == "null" ]; then continue; fi
        
        # Garante que o caminho seja absoluto ou relativo à raiz do projeto
        # Esta é uma simplificação; para robustez, caminhos absolutos são melhores.
        
        if [ -d "$dir_path" ]; then
            log "INFO" "Diretório '$dir_path' já existe."
        else
            log "INFO" "Criando diretório '$dir_path'..."
            if mkdir -p "$dir_path"; then
                log "SUCCESS" "Diretório '$dir_path' criado com sucesso."
            else
                log "ERROR" "Falha ao criar o diretório '$dir_path'."
            fi
        fi
    done
}

check_and_set_wordlist_dir() {
    if ! command -v jq &> /dev/null; then
        echo -e "${RED}Erro: 'jq' não encontrado. Esta função requer jq.${NC}"
        return 1
    fi

    if [ ! -f "$ENV_JSON_PATH" ]; then
        echo -e "${RED}Erro: Arquivo de configuração '$ENV_JSON_PATH' não encontrado.${NC}"
        return 1
    fi

    local current_wordlist_dir
    current_wordlist_dir=$(jq -r '.path.wordlist_dir' "$ENV_JSON_PATH")

    if [ -n "$current_wordlist_dir" ] && [ -d "$current_wordlist_dir" ]; then
        echo -e "${GREEN}Diretório de wordlists já configurado em: '$current_wordlist_dir'${NC}"
        # Se o diretório existe, mas o mapa não, ou o mapa está desatualizado, podemos regenerá-lo.
        # Por simplicidade, vamos regenerar sempre que o diretório for confirmado.
        generate_wordlist_map_from_dir "$current_wordlist_dir"
        return 0
    fi

    echo -e "\n${YELLOW}O diretório de wordlists não está configurado ou não foi encontrado.${NC}"
    read -p "Por favor, informe o diretório base para as wordlists (ex: /usr/share/wordlists): " new_wordlist_dir

    if [ -n "$new_wordlist_dir" ]; then
        update_json_value "$ENV_JSON_PATH" ".path.wordlist_dir" "$new_wordlist_dir"
        mkdir -p "$new_wordlist_dir" # Garante que o diretório seja criado
        generate_wordlist_map_from_dir "$new_wordlist_dir" # Gera o mapa após definir o diretório
    fi
}

# Helper para obter o caminho do arquivo JSON de wordlists
get_wordlist_map_file_path() {
    local map_path
    map_path=$(jq -r '.path.wordlists_map_file' "$ENV_JSON_PATH" 2>/dev/null)
    if [ "$map_path" = "null" ] || [ -z "$map_path" ]; then
        # Fallback: se não estiver em env.json, tenta deduzir
        local wordlist_dir=$(jq -r '.path.wordlist_dir' "$ENV_JSON_PATH" 2>/dev/null)
        if [ "$wordlist_dir" != "null" ] && [ -n "$wordlist_dir" ]; then
            echo "$wordlist_dir/autohunting_wordlists_map.json"
        else
            echo "" # Não foi possível determinar
        fi
    else
        echo "$map_path"
    fi
}

# Adiciona uma wordlist ao JSON de mapa
add_wordlist_to_map() {
    local name="$1"
    local path="$2"
    local map_file=$(get_wordlist_map_file_path)

    if [ -z "$map_file" ]; then
        echo -e "${RED}Erro: Não foi possível determinar o caminho do arquivo de mapa de wordlists.${NC}"
        return 1
    fi

    if [ ! -f "$ENV_JSON_PATH" ]; then
        echo -e "${RED}Erro: Arquivo de configuração '$ENV_JSON_PATH' não encontrado.${NC}"
        return 1
    fi

    # Garante que o diretório do mapa exista
    mkdir -p "$(dirname "$map_file")"

    local current_json="{\"wordlists\":[]}"
    if [ -f "$map_file" ]; then
        current_json=$(cat "$map_file")
    fi

    local new_entry=$(jq -n --arg name "$name" --arg path "$path" '{"name": $name, "path": $path}')
    local updated_json=$(echo "$current_json" | jq --argjson new_entry "$new_entry" '.wordlists |= (. + [$new_entry]) | unique_by(.name)')

    echo "$updated_json" > "$map_file"
    echo -e "${GREEN}Wordlist '$name' adicionada/atualizada no mapa: $map_file${NC}"
}

# Gera o mapa de wordlists a partir de um diretório
generate_wordlist_map_from_dir() {
    local wordlist_base_dir="$1"
    local wordlists_map_file=$(get_wordlist_map_file_path)

    if [ -z "$wordlists_map_file" ]; then
        echo -e "${RED}Erro: Não foi possível determinar o caminho do arquivo de mapa de wordlists.${NC}"
        return 1
    fi

    # Garante que o diretório do mapa exista
    mkdir -p "$(dirname "$wordlists_map_file")"

    local json_output="{\"wordlists\":[]}"

    echo -e "\n[*] Gerando mapa de wordlists em '$wordlists_map_file' a partir de '$wordlist_base_dir'..."

    # Find .txt files and build JSON array
    find "$wordlist_base_dir" -type f -name "*.txt" -print0 | while IFS= read -r -d '' file; do
        local name=$(basename "$file" .txt)
        local escaped_file=$(echo "$file" | sed 's/"/\\"/g') # Escape double quotes in path
        local escaped_name=$(echo "$name" | sed 's/"/\\"/g') # Escape double quotes in name
        json_output=$(echo "$json_output" | jq --arg name "$escaped_name" --arg path "$escaped_file" '.wordlists += [{"name": $name, "path": $path}]')
    done

    # Remove duplicates by name
    json_output=$(echo "$json_output" | jq '.wordlists |= unique_by(.name)')

    echo "$json_output" > "$wordlists_map_file"

    # Atualiza env.json com o caminho para o arquivo de mapa, se ainda não estiver lá
    local current_map_path=$(jq -r '.path.wordlists_map_file' "$ENV_JSON_PATH" 2>/dev/null)
    if [ "$current_map_path" = "null" ] || [ -z "$current_map_path" ] || [ "$current_map_path" != "$wordlists_map_file" ]; then
        update_json_value "$ENV_JSON_PATH" ".path.wordlists_map_file" "$wordlists_map_file"
    fi

    echo -e "${GREEN}Mapa de wordlists gerado com sucesso em: $wordlists_map_file${NC}"
}


update_json_value() { # Mantida a função original
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

check_and_set_wordlist_dir # Adiciona a verificação inicial aqui

while true; do
    show_menu_principal
    read -p "Escolha uma opção: " opcao

    case $opcao in
        1)
            echo -e "\n[*] Verificando e recriando diretórios base..."
            verificar_e_criar_diretorios_base
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
        5)
            while true; do
                show_menu_retention
                read -p "Escolha uma opção: " retention_opcao
                case $retention_opcao in
                    1)
                        enable_retention_cron
                        ;;
                    2)
                        disable_retention_cron
                        ;;
                    3)
                        read -p "Dias de retenção [atual: $RETENTION_DAYS]: " days
                        RETENTION_DAYS="${days:-$RETENTION_DAYS}"
                        log "INFO" "RETENTION_DAYS definido para $RETENTION_DAYS"
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
                        echo -e "\n${YELLOW}A verificação e criação de templates agora é feita pelo 'install.sh'.${NC}"
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
                                5) 
                                    while true; do
                                        show_menu_Wordlist
                                        read -p "Escolha uma opção para Wordlists: " wordlist_opcao
                                        case $wordlist_opcao in
                                            1)
                                                read -p "Deseja o modo silencioso (salva o caminho com o nome do arquivo) ou assistido (pergunta um nome para cada)? (s/A): " modo_wordlist
                                                echo -e "[*] Digite 'sair' para terminar."

                                                while true; do
                                                    read -p "Informe o caminho para o arquivo .txt da wordlist: " wordlist_path
                                                    if [[ "$wordlist_path" == "sair" ]]; then
                                                        break
                                                    fi

                                                    if [[ ! "$wordlist_path" == *.txt ]]; then
                                                        echo -e "${RED}Erro: O caminho deve terminar com .txt${NC}"
                                                        continue
                                                    fi

                                                    if [ ! -f "$wordlist_path" ]; then
                                                        echo -e "${RED}Erro: Arquivo não encontrado em '$wordlist_path'${NC}"
                                                        continue
                                                    fi

                                                    local wordlist_name
                                                    wordlist_name=$(basename "$wordlist_path" .txt)

                                                    if [[ ! "$modo_wordlist" =~ ^[sS]$ ]]; then
                                                        # Modo Assistido
                                                        read -p "Digite um nome para esta wordlist (padrão: '$wordlist_name'): " custom_name
                                                        if [ -n "$custom_name" ]; then
                                                            wordlist_name="$custom_name"
                                                        fi
                                                        add_wordlist_to_map "$wordlist_name" "$wordlist_path"
                                                        echo -e "${GREEN}Wordlist '$wordlist_name' adicionada ao mapa.${NC}"
                                                    else
                                                        # Modo Silencioso
                                                        add_wordlist_to_map "$wordlist_name" "$wordlist_path"
                                                        echo -e "${GREEN}Wordlist '$wordlist_name' adicionada ao mapa (modo silencioso).${NC}"
                                                    fi
                                                done
                                                echo -e "\n${GREEN}Configuração manual de wordlists concluída.${NC}"
                                                break
                                                ;;
                                            2)
                                                echo -e "\n[*] Iniciando busca automática por arquivos .txt no diretório base: '$current_wordlist_dir'..."
                                                generate_wordlist_map_from_dir "$current_wordlist_dir"
                                                break # Sai do loop de wordlist
                                                ;;
                                            0) break ;; # Sai do loop de wordlist
                                            *) echo -e "\n[!] Opção inválida.\n" ;;
                                        esac
                                    done
                                    continue # Volta para o menu de caminhos
                                    ;;
                                6) key_to_update="log_dir" ;;
                                0) break ;;
                                *) echo -e "\n[!] Opção inválida.\n"; continue ;;
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
                    6) 
                        while true; do
                            show_menu_logs
                            read -p "Escolha uma opção para Logs: " log_opcao
                            case $log_opcao in
                                1)
                                    read -p "Digite o novo caminho para o diretório de logs: " new_log_path
                                    if [ -n "$new_log_path" ]; then
                                        update_json_value "$ENV_JSON_PATH" ".path.log_dir" "$new_log_path"
                                    else
                                        echo -e "${YELLOW}Nenhum caminho fornecido. Operação cancelada.${NC}"
                                    fi
                                    ;;
                                2)
                                    read -p "Deseja que o log seja salvo em arquivo ou exibido no terminal? (Arquivo/Terminal): " tipo_log
                                    if [[ "$tipo_log" =~ ^[Tt] ]]; then
                                        local terminal_shell
                                        terminal_shell=$(echo "$SHELL")
                                        local terminal_path
                                        terminal_path=$(command -v "$(basename "$terminal_shell")")
                                        echo -e "[*] Saída configurada para o terminal."
                                        echo -e "    Seu terminal atual é: ${BOLD}${terminal_shell}${NC}"
                                        echo -e "    Localizado em: ${BOLD}${terminal_path}${NC}"
                                        # Aqui você pode adicionar lógica para salvar essa preferência, se necessário
                                    else
                                        echo -e "[*] Saída configurada para salvar em arquivo (padrão)."
                                    fi
                                    ;;
                                3)
                                    echo -e "\n${YELLOW}Funcionalidade de política de retenção de logs ainda não implementada.${NC}"
                                    ;;
                                0) break ;;
                                *) echo -e "\n[!] Opção inválida.\n" ;;
                            esac
                        done
                        ;;
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

                                                    if [[ ! "$wordlist_path" == *.txt ]]; then
                                                        echo -e "${RED}Erro: O caminho deve terminar com .txt${NC}"
                                                        continue
                                                    fi

                                                    if [ ! -f "$wordlist_path" ]; then
                                                        echo -e "${RED}Erro: Arquivo não encontrado em '$wordlist_path'${NC}"
                                                        continue
                                                    fi

                                                    local wordlist_name
                                                    wordlist_name=$(basename "$wordlist_path" .txt)

                                                    if [[ ! "$modo_wordlist" =~ ^[sS]$ ]]; then
                                                        # Modo Assistido
                                                        read -p "Digite um nome para esta wordlist (padrão: '$wordlist_name'): " custom_name
                                                        if [ -n "$custom_name" ]; then
                                                            wordlist_name="$custom_name"
                                                        fi
                                                        echo "$wordlist_name:$wordlist_path" >> "$output_file"
                                                        echo -e "${GREEN}Wordlist '$wordlist_name' adicionada.${NC}"
                                                    else
                                                        # Modo Silencioso
                                                        echo "$wordlist_name:$wordlist_path" >> "$output_file"
                                                        echo -e "${GREEN}Wordlist '$wordlist_name' adicionada (modo silencioso).${NC}"
                                                    fi
                                                done
                                                echo -e "\n${GREEN}Configuração manual de wordlists concluída. Arquivo de referência: $output_file${NC}"
                                                echo -e "Para usar esses nomes no 'commands.json', você pode referenciá-los no futuro."
                                                # A lógica de definir um único diretório foi removida para dar lugar a esta mais flexível.
                                                # Se ainda precisar, pode ser adicionada como uma opção separada.
                                                break
                                                ;;
                                            2)
                                                echo -e "\n[*] Iniciando busca automática por arquivos .txt..."
                                                local output_file="/tmp/autohunting_wordlists_auto.txt"
                                                : > "$output_file" # Limpa o arquivo antes de começar
                                                
                                                find . -type f -name "*.txt" -print0 | while IFS= read -r -d '' file; do
                                                    echo "Encontrado: $file"
                                                    local name
                                                    name=$(basename "$file" .txt)
                                                    echo "$name:$file" >> "$output_file"
                                                done
                                                
                                                echo -e "\n${GREEN}Busca concluída. Os caminhos para as wordlists foram salvos em: $output_file${NC}"
                                                echo -e "Você pode usar este arquivo como referência para configurar suas ferramentas."
                                                break # Sai do loop de wordlist
                                                ;;
                                            0) break ;; # Sai do loop de wordlist
                                            *) echo -e "\n[!] Opção inválida.\n" ;;
                                        esac
                                    done
                                    continue # Volta para o menu de caminhos
                                    ;;
                                6) key_to_update="log_dir" ;;
                                0) break ;;
                                *) echo -e "\n[!] Opção inválida.\n"; continue ;;
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
                    6) 
                        while true; do
                            show_menu_logs
                            read -p "Escolha uma opção para Logs: " log_opcao
                            case $log_opcao in
                                1)
                                    read -p "Digite o novo caminho para o diretório de logs: " new_log_path
                                    if [ -n "$new_log_path" ]; then
                                        update_json_value "$ENV_JSON_PATH" ".path.log_dir" "$new_log_path"
                                    else
                                        echo -e "${YELLOW}Nenhum caminho fornecido. Operação cancelada.${NC}"
                                    fi
                                    ;;
                                2)
                                    read -p "Deseja que o log seja salvo em arquivo ou exibido no terminal? (Arquivo/Terminal): " tipo_log
                                    if [[ "$tipo_log" =~ ^[Tt] ]]; then
                                        local terminal_shell
                                        terminal_shell=$(echo "$SHELL")
                                        local terminal_path
                                        terminal_path=$(command -v "$(basename "$terminal_shell")")
                                        echo -e "[*] Saída configurada para o terminal."
                                        echo -e "    Seu terminal atual é: ${BOLD}${terminal_shell}${NC}"
                                        echo -e "    Localizado em: ${BOLD}${terminal_path}${NC}"
                                        # Aqui você pode adicionar lógica para salvar essa preferência, se necessário
                                    else
                                        echo -e "[*] Saída configurada para salvar em arquivo (padrão)."
                                    fi
                                    ;;
                                3)
                                    echo -e "\n${YELLOW}Funcionalidade de política de retenção de logs ainda não implementada.${NC}"
                                    ;;
                                0) break ;;
                                *) echo -e "\n[!] Opção inválida.\n" ;;
                            esac
                        done
                        ;;
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
                                                fi
                                                break # Sai do loop de wordlist
                                                ;;
                                            2)
                                                echo -e "\n[*] Iniciando busca automática por arquivos .txt..."
                                                local output_file="/tmp/autohunting_wordlists.txt"
                                                : > "$output_file" # Limpa o arquivo antes de começar
                                                
                                                find . -type f -name "*.txt" -print0 | while IFS= read -r -d '' file; do
                                                    echo "Encontrado: $file"
                                                    echo "$file" >> "$output_file"
                                                done
                                                
                                                echo -e "\n${GREEN}Busca concluída. Os caminhos para as wordlists foram salvos em: $output_file${NC}"
                                                echo -e "Você pode usar este arquivo como referência para configurar suas ferramentas."
                                                break # Sai do loop de wordlist
                                                ;;
                                            0) break ;; # Sai do loop de wordlist
                                            *) echo -e "\n[!] Opção inválida.\n" ;;
                                        esac
                                    done
                                    continue # Volta para o menu de caminhos
                                    ;;
                                6) key_to_update="log_dir" ;;
                                0) break ;;
                                *) echo -e "\n[!] Opção inválida.\n"; continue ;;
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
        7)
            handle_immunize_menu
            ;;
        0) echo "Saindo..."; exit 0 ;;
        *)
            echo -e "\n[!] Opção inválida.\n"
            ;;
    esac
done
