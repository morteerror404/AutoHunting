#!/usr/bin/env bash
set -euo pipefail

# ========================================
# db_config.sh - Configuração de Bancos
# (com criação de usuários, Metasploit integ.,
#  e retenção/cron (desativada por padrão))
# ========================================

# Colors
BOLD="\033[1m"
GREEN="\033[0;32m"
RED="\033[0;31m"
YELLOW="\033[1;33m"
BLUE="\033[0;34m"
NC="\033[0m"

# Globals
DEFAULT_LOG="/var/log/db_config.log"
if [ -w "$(dirname "$DEFAULT_LOG")" ] 2>/dev/null || [ "$(id -u)" -eq 0 ]; then
    LOG_FILE="$DEFAULT_LOG"
else
    LOG_FILE="$HOME/.db_config.log"
fi

RETENTION_DAYS=30
SERVICE_CMD="$(command -v systemctl || command -v service || true)"
SELECTED_DB=""
AUTODIR="/var/lib/autohunt"
MARKER_DIR="$AUTODIR/markers"
CRED_DIR="$AUTODIR/creds"
RETENTION_SCRIPT="/usr/local/bin/autohunt_retention_cleanup.sh"
CRON_MARKER="/etc/cron.d/autohunt_retention"   # or keep in root crontab
MANAGER_TOOL="ManagerDB.sh"                   # nome conveniente para referencia
LOGGER_USER="autohunt_logger"

# Ensure directories
mkdir -p "$MARKER_DIR" "$CRED_DIR" 2>/dev/null || true
chmod 700 "$CRED_DIR" 2>/dev/null || true

# ---------- Logging utilities ----------
verifica_basico() {
    if [ "$(id -u)" -ne 0 ] && [ -z "${SUDO_USER:-}" ]; then
        log "ERROR" "Erro: Execute como root (sudo)!"
        echo -e "${RED}Erro: Execute como root (sudo)!${NC}"
        exit 1
    fi
    log "INFO" "Permissões ok (root or sudo)."
}

configurar_log() {
    local logdir
    logdir="$(dirname "$LOG_FILE")"
    if ! mkdir -p "$logdir" 2>/dev/null; then
        LOG_FILE="$HOME/.db_config.log"
        mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null || true
    fi
    touch "$LOG_FILE" 2>/dev/null || true
    exec > >(tee -a "$LOG_FILE") 2>&1
    echo -e "${BLUE}Log iniciado: $LOG_FILE${NC}"
    log "INFO" "Log configurado: $LOG_FILE"
}

log() {
    local level="$1"; shift
    printf '[%s] [%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$level" "$*" | tee -a "$LOG_FILE"
}

pause() { read -rp "Pressione ENTER para continuar..." _; }

check_command() {
    if ! command -v "$1" &>/dev/null; then
        log "WARN" "Comando '$1' não encontrado."
        return 1
    fi
    return 0
}

random_password() {
    # strong password ~24 chars
    head -c 48 /dev/urandom | tr -dc 'A-Za-z0-9!@%_+' | head -c 24 || echo "autohunt_default_pass"
}

# ---------- DB install helpers ----------
install_postgresql() {
    log "INFO" "Instalando PostgreSQL..."
    if check_command apt-get; then
        apt-get update
        apt-get install -y postgresql postgresql-contrib
    elif check_command dnf; then
        dnf install -y postgresql-server postgresql-contrib
    elif check_command yum; then
        yum install -y postgresql-server postgresql-contrib
    else
        log "ERROR" "Nenhum gerenciador de pacotes detectado para PostgreSQL."
        return 1
    fi

    # initdb on some distros might be needed
    if [ -n "$SERVICE_CMD" ]; then
        $SERVICE_CMD enable postgresql 2>/dev/null || true
        $SERVICE_CMD start postgresql 2>/dev/null || true
    fi
    log "SUCCESS" "PostgreSQL tentado instalar e iniciar."
}

install_mariadb() {
    log "INFO" "Instalando MariaDB..."
    if check_command apt-get; then
        apt-get update
        apt-get install -y mariadb-server mariadb-client
    elif check_command dnf; then
        dnf install -y mariadb-server mariadb
    elif check_command yum; then
        yum install -y mariadb-server mariadb
    else
        log "ERROR" "Nenhum gerenciador de pacotes detectado para MariaDB."
        return 1
    fi

    if [ -n "$SERVICE_CMD" ]; then
        $SERVICE_CMD enable mariadb 2>/dev/null || true
        $SERVICE_CMD start mariadb 2>/dev/null || true
    fi
    log "SUCCESS" "MariaDB tentado instalar e iniciar."
}

install_mysql() {
    log "INFO" "Instalando MySQL..."
    if check_command apt-get; then
        apt-get update
        apt-get install -y mysql-server mysql-client
    elif check_command dnf; then
        dnf install -y mysql-server
    elif check_command yum; then
        yum install -y mysql-server
    else
        log "ERROR" "Nenhum gerenciador de pacotes detectado para MySQL."
        return 1
    fi

    if [ -n "$SERVICE_CMD" ]; then
        $SERVICE_CMD enable mysql 2>/dev/null || true
        $SERVICE_CMD start mysql 2>/dev/null || true
    fi
    log "SUCCESS" "MySQL tentado instalar e iniciar."
}

install_mongodb() {
    log "INFO" "Instalando MongoDB..."
    if check_command apt-get; then
        apt-get update
        apt-get install -y mongodb || apt-get install -y mongodb-org || true
    elif check_command dnf; then
        dnf install -y mongodb-server || true
    elif check_command yum; then
        yum install -y mongodb-server || true
    else
        log "ERROR" "Nenhum gerenciador de pacotes detectado para MongoDB."
        return 1
    fi

    if [ -n "$SERVICE_CMD" ]; then
        $SERVICE_CMD enable mongod 2>/dev/null || $SERVICE_CMD enable mongodb 2>/dev/null || true
        $SERVICE_CMD start mongod 2>/dev/null || $SERVICE_CMD start mongodb 2>/dev/null || true
    fi
    log "SUCCESS" "MongoDB tentado instalar e iniciar."
}

# ---------- Create DB and marker ----------
create_bughunt_db() {
    local db="$1"   # values: postgresql, mariadb, mysql, mongodb
    log "INFO" "Criando banco bughunt no $db (se não existir) e registrando marker."

    case "$db" in
        postgresql)
            if command -v psql >/dev/null 2>&1; then
                sudo -u postgres psql -c "CREATE DATABASE IF NOT EXISTS bughunt;" >/dev/null 2>&1 || sudo -u postgres psql -c "SELECT 1 FROM pg_database WHERE datname='bughunt';" >/dev/null 2>&1 || true
            else
                log "ERROR" "psql não encontrado."
            fi
            ;;
        mariadb|mysql)
            if command -v mysql >/dev/null 2>&1; then
                mysql -u root -e "CREATE DATABASE IF NOT EXISTS bughunt;" || log "WARN" "Falha ao criar DB via mysql client."
            else
                log "ERROR" "mysql client não encontrado."
            fi
            ;;
        mongodb)
            if command -v mongo >/dev/null 2>&1; then
                mongo --eval 'db.getSiblingDB("bughunt")' >/dev/null 2>&1 || true
            else
                log "ERROR" "mongo client não encontrado."
            fi
            ;;
        *)
            log "ERROR" "DB desconhecido: $db"
            return 1
            ;;
    esac

    # create marker file for retention tracking
    local marker_file="$MARKER_DIR/bughunt.$db.marker"
    date +%s > "$marker_file"
    log "INFO" "Marker criado: $marker_file (timestamp $(date -d @$(cat "$marker_file") 2>/dev/null || date))"
    chmod 600 "$marker_file" 2>/dev/null || true
}

# ---------- Create logger user (separate from Metasploit) ----------
create_logger_user() {
    local db="$1"
    local pass
    pass="$(random_password)"
    mkdir -p "$CRED_DIR" 2>/dev/null || true

    case "$db" in
        postgresql)
            if command -v psql >/dev/null 2>&1; then
                sudo -u postgres psql -c "DO \$\$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${LOGGER_USER}') THEN CREATE ROLE ${LOGGER_USER} LOGIN PASSWORD '${pass}'; END IF; END\$\$;" >/dev/null 2>&1 || true
                sudo -u postgres psql -c "GRANT CONNECT ON DATABASE bughunt TO ${LOGGER_USER};" >/dev/null 2>&1 || true
                sudo -u postgres psql -d bughunt -c "GRANT INSERT ON ALL TABLES IN SCHEMA public TO ${LOGGER_USER};" >/dev/null 2>&1 || true
            else
                log "ERROR" "psql não encontrado; usuário logger não criado (Postgres)."
            fi
            ;;
        mariadb|mysql)
            if command -v mysql >/dev/null 2>&1; then
                mysql -u root -e "CREATE USER IF NOT EXISTS '${LOGGER_USER}'@'localhost' IDENTIFIED BY '${pass}'; GRANT INSERT ON bughunt.* TO '${LOGGER_USER}'@'localhost'; FLUSH PRIVILEGES;" >/dev/null 2>&1 || true
            else
                log "ERROR" "mysql client não encontrado; usuário logger não criado (MySQL/MariaDB)."
            fi
            ;;
        mongodb)
            if command -v mongo >/dev/null 2>&1; then
                # create user in bughunt db with readWrite role
                mongo bughunt --eval "db.createUser({user: '${LOGGER_USER}', pwd: '${pass}', roles: [{role: 'readWrite', db: 'bughunt'}]})" >/dev/null 2>&1 || true
            else
                log "ERROR" "mongo client não encontrado; usuário logger não criado (MongoDB)."
            fi
            ;;
        *)
            log "ERROR" "DB desconhecido para criar logger user: $db"
            return 1
            ;;
    esac

    # Store credentials securely
    local cred_file="$CRED_DIR/${LOGGER_USER}_${db}.txt"
    {
        echo "db=$db"
        echo "user=${LOGGER_USER}"
        echo "pass=${pass}"
        echo "created=$(date --iso-8601=seconds 2>/dev/null || date)"
    } > "$cred_file"
    chmod 600 "$cred_file" 2>/dev/null || true
    log "INFO" "Usuário logger criado para $db. Credenciais em: $cred_file"
}

# ---------- Keep Metasploit user intact, but create msf DB if asked ----------
ensure_metasploit_db() {
    # create msf DB only for PostgreSQL and MySQL variants (msf supports pg/mysql)
    local db="$1"
    case "$db" in
        postgresql)
            if command -v psql >/dev/null 2>&1; then
                sudo -u postgres psql -c "DO \$\$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'msf') THEN CREATE USER msf WITH PASSWORD 'msf'; END IF; END\$\$;" >/dev/null 2>&1 || true
                sudo -u postgres psql -c "CREATE DATABASE IF NOT EXISTS msf;" >/dev/null 2>&1 || true
                log "INFO" "Metasploit DB/user ensured on PostgreSQL (user 'msf' untouched if exists)."
            fi
            ;;
        mariadb|mysql)
            if command -v mysql >/dev/null 2>&1; then
                mysql -u root -e "CREATE DATABASE IF NOT EXISTS msf; CREATE USER IF NOT EXISTS 'msf'@'localhost' IDENTIFIED BY 'msf'; GRANT ALL PRIVILEGES ON msf.* TO 'msf'@'localhost'; FLUSH PRIVILEGES;" >/dev/null 2>&1 || true
                log "INFO" "Metasploit DB/user ensured on MySQL/MariaDB (user 'msf' untouched if exists)."
            fi
            ;;
        *)
            log "WARN" "Metasploit DB not applicable for $db"
            ;;
    esac
}

# ---------- Retention cleanup script (writes to /usr/local/bin) ----------
install_retention_script() {
    log "INFO" "Escrevendo script de retenção em $RETENTION_SCRIPT"
    cat > "$RETENTION_SCRIPT" <<'EOF'
#!/usr/bin/env bash
# autohunt_retention_cleanup.sh
# Executa remoção de DBs marcados que excederam a retenção.

MARKER_DIR="/var/lib/autohunt/markers"
RETENTION_DAYS_DEFAULT=30
LOGFILE="/var/log/db_config.log"

log() { printf '[%s] [%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$1" "$2" | tee -a "$LOGFILE"; }

RETENTION_DAYS="${RETENTION_DAYS:-$RETENTION_DAYS_DEFAULT}"

for marker in "$MARKER_DIR"/*.marker 2>/dev/null; do
    [ -f "$marker" ] || continue
    # marker name bughunt.<db>.marker or custom: <name>.<type>.marker
    stamp=$(cat "$marker" 2>/dev/null || echo 0)
    if ! [[ "$stamp" =~ ^[0-9]+$ ]]; then
        log "WARN" "Marker $marker invalid timestamp"
        continue
    fi
    age_days=$(( ( $(date +%s) - stamp ) / 86400 ))
    # find db and type from filename
    base=$(basename "$marker")
    name="${base%%.*}"    # before first dot (bughunt)
    dbtype="${base#*.}"   # remainder: <db>.marker
    dbtype="${dbtype%.marker}"

    if [ "$age_days" -ge "$RETENTION_DAYS" ]; then
        log "INFO" "Marker $marker age $age_days >= $RETENTION_DAYS: removing database $name for $dbtype"
        case "$dbtype" in
            postgresql)
                sudo -u postgres psql -c "DROP DATABASE IF EXISTS ${name};" 2>/dev/null || log "ERROR" "Failed drop ${name} on postgres"
                ;;
            mariadb|mysql)
                mysql -u root -e "DROP DATABASE IF EXISTS \`${name}\`;" 2>/dev/null || log "ERROR" "Failed drop ${name} on mysql/mariadb"
                ;;
            mongodb)
                mongo --eval "db.getSiblingDB('${name}').dropDatabase()" 2>/dev/null || log "ERROR" "Failed drop ${name} on mongodb"
                ;;
            *)
                log "WARN" "Unknown db type: $dbtype for marker $marker"
                ;;
        esac
        # archive or remove marker
        mv "$marker" "${marker}.removed.$(date +%s)" 2>/dev/null || rm -f "$marker" 2>/dev/null
        log "INFO" "Database ${name} removal attempted and marker archived/removed."
    else
        # nothing to do
        log "DEBUG" "Marker $marker age $age_days (<$RETENTION_DAYS) -> keep"
    fi
done
EOF
    chmod 750 "$RETENTION_SCRIPT"
    log "INFO" "Script de retenção escrito e autorizado em $RETENTION_SCRIPT"
}

# ---------- Enable/disable cron job ----------
enable_retention_cron() {
    if [ ! -f "$RETENTION_SCRIPT" ]; then
        install_retention_script
    fi

    read -rp "Deseja ativar a cron job diária de retenção que irá excluir DBs expirados? (s/N): " resp
    if [[ ! "$resp" =~ ^[sS]$ ]]; then
        log "INFO" "Usuário optou por não ativar cron de retenção."
        return
    fi

    # create cron file (runs as root daily at 03:30)
    cat > "$CRON_MARKER" <<EOF
# Autohunt retention cron - daily at 03:30
30 3 * * * root $RETENTION_SCRIPT
EOF
    log "SUCCESS" "Cron de retenção ativado: $CRON_MARKER"
    # Install profile warning for users
    install_terminal_warning
}

disable_retention_cron() {
    if [ -f "$CRON_MARKER" ]; then
        rm -f "$CRON_MARKER"
        log "INFO" "Cron de retenção removido: $CRON_MARKER"
    else
        log "INFO" "Cron de retenção não estava ativo."
    fi
    remove_terminal_warning
}

# ---------- Terminal warning (shows remaining time until deletion) ----------
install_terminal_warning() {
    # creates /etc/profile.d/autohunt_retention_warning.sh to show a message at login
    local warnfile="/etc/profile.d/autohunt_retention_warning.sh"
    cat > "$warnfile" <<'EOF'
#!/usr/bin/env bash
MARKER_DIR="/var/lib/autohunt/markers"
MANAGER="ManagerDB.sh"
for m in "$MARKER_DIR"/*.marker 2>/dev/null; do
  [ -f "$m" ] || continue
  name="$(basename "$m")"
  # format: bughunt.postgresql.marker
  base="${name%.marker}"
  db="$(echo "$base" | awk -F. '{print $2}')"
  item="$(echo "$base" | awk -F. '{print $1}')"
  stamp=$(cat "$m" 2>/dev/null || echo 0)
  if ! [[ "$stamp" =~ ^[0-9]+$ ]]; then continue; fi
  # retention days from env or default
  RETENTION_DAYS=${RETENTION_DAYS:-30}
  expiry=$((stamp + (RETENTION_DAYS * 86400)))
  now=$(date +%s)
  if [ "$expiry" -gt "$now" ]; then
    seconds_left=$((expiry - now))
    days_left=$((seconds_left / 86400))
    hours_left=$(((seconds_left % 86400) / 3600))
    echo -e "\033[1;33mAVISO AUTOhunt: O banco/tabela '\033[1;31m$item\033[0;33m' (tipo: \033[1;31m$db\033[0;33m) será excluído em \033[1;31m${days_left}\033[0;33m dias e \033[1;31m${hours_left}\033[0;33m horas\033[0m"
    echo -e "Para imunizar esse banco/tabela execute: $MANAGER (ex: sudo ./$MANAGER immunize $item $db)"
    echo
  fi
done
EOF
    chmod 644 "$warnfile" 2>/dev/null || true
    log "INFO" "Aviso de retenção instalado em $warnfile (aparecerá em novos logins)."
}

remove_terminal_warning() {
    local warnfile="/etc/profile.d/autohunt_retention_warning.sh"
    rm -f "$warnfile" 2>/dev/null || true
    log "INFO" "Aviso de retenção removido ($warnfile)."
}

# ---------- Immunize a DB/TABLE (creates .immunized file) ----------
immunize_item() {
    # usage: immunize_item <name> <dbtype>
    local name="$1"
    local dbtype="$2"
    local immun_file="$MARKER_DIR/${name}.${dbtype}.immunized"
    date +%s > "$immun_file"
    chmod 600 "$immun_file" 2>/dev/null || true
    log "INFO" "Item imunizado: $name (type $dbtype). Marker: $immun_file"
}

# ---------- Check whether a marker is immunized ----------
is_immunized() {
    local name="$1"; local dbtype="$2"
    local immun_file="$MARKER_DIR/${name}.${dbtype}.immunized"
    [ -f "$immun_file" ] && return 0 || return 1
}

# ---------- Generic configuration menu (per DB) ----------
config_generic_menu() {
    while true; do
        echo -e "${BOLD}${GREEN}=== Configuração do $SELECTED_DB ===${NC}"
        echo "1) Criar serviço de inicialização do $SELECTED_DB junto com o sistema"
        echo "2) Criar banco 'bughunt' e registrar marker"
        echo "3) Criar usuário logger (autohunt_logger) para esse DB"
        echo "4) Conectar automaticamente ao Metasploit (mantém usuário msf padrão)"
        echo "5) Configurar política de retenção (RETENTION_DAYS)"
        echo "6) Habilitar cron de retenção (desativado por padrão)"
        echo "7) Desabilitar cron de retenção"
        echo "8) Imunizar banco/tabela (evita exclusão pelo cron)"
        echo "0) Voltar"
        read -rp "Escolha: " opt

        case "$opt" in
            1)
                case "$SELECTED_DB" in
                    MariaDB) [ -n "$SERVICE_CMD" ] && $SERVICE_CMD enable mariadb 2>/dev/null || true ;;
                    MySQL)   [ -n "$SERVICE_CMD" ] && $SERVICE_CMD enable mysql 2>/dev/null || true ;;
                    MongoDB) [ -n "$SERVICE_CMD" ] && $SERVICE_CMD enable mongod 2>/dev/null || $SERVICE_CMD enable mongodb 2>/dev/null || true ;;
                    PostgreSQL) [ -n "$SERVICE_CMD" ] && $SERVICE_CMD enable postgresql 2>/dev/null || true ;;
                esac
                log "INFO" "$SELECTED_DB configurado para iniciar com o sistema (se disponível)."
                ;;
            2)
                create_bughunt_db "$SELECTED_DB"
                ;;
            3)
                create_logger_user "$SELECTED_DB"
                ;;
            4)
                # Ensure msf db/user but do not change existing msf user if present
                ensure_metasploit_db "$SELECTED_DB"
                connect_metasploit_prompt
                ;;
            5)
                read -rp "Dias de retenção [atual: $RETENTION_DAYS]: " days
                RETENTION_DAYS="${days:-$RETENTION_DAYS}"
                log "INFO" "RETENTION_DAYS definido para $RETENTION_DAYS"
                ;;
            6)
                enable_retention_cron
                ;;
            7)
                disable_retention_cron
                ;;
            8)
                read -rp "Nome do banco/tabela a imunizar (ex: bughunt): " item
                immunize_item "$item" "$SELECTED_DB"
                ;;
            0) break ;;
            *) echo "Opção inválida" ;;
        esac
    done
}

# ---------- Metasploit connect prompt (keeps msf user) ----------
connect_metasploit_prompt() {
    read -rp "Deseja criar/atualizar arquivo de configuração do Metasploit (~/.msf4/database.yml) para $SELECTED_DB? (s/N): " resp
    if [[ "$resp" =~ ^[sS]$ ]]; then
        connect_metasploit "$SELECTED_DB"
    else
        log "INFO" "Integração Metasploit ignorada pelo usuário."
    fi
}

# ---------- Metasploit configuration helper ----------
connect_metasploit() {
    local msf_dir="$HOME/.msf4"
    mkdir -p "$msf_dir" 2>/dev/null || true
    case "$SELECTED_DB" in
        MariaDB|MySQL)
            cat >"$msf_dir/database.yml" <<EOF
production:
  adapter: mysql2
  database: bughunt
  username: root
  password:
  host: 127.0.0.1
  port: 3306
EOF
            ;;
        PostgreSQL)
            cat >"$msf_dir/database.yml" <<EOF
production:
  adapter: postgresql
  database: bughunt
  username: postgres
  password:
  host: 127.0.0.1
  port: 5432
EOF
            ;;
        MongoDB)
            log "WARN" "Metasploit não suporta MongoDB para o DB interno; integração não aplicável."
            return
            ;;
        *)
            log "ERROR" "Tipo de DB desconhecido para Metasploit: $SELECTED_DB"
            return
            ;;
    esac
    chmod 600 "$msf_dir/database.yml" 2>/dev/null || true
    log "SUCCESS" "Arquivo ~/.msf4/database.yml criado para $SELECTED_DB. (Usuário 'msf' não alterado.)"
}

# ---------- Select and installation menu ----------
select_db_menu() {
    while true; do
        echo -e "${BOLD}${GREEN}=== Seleção de Banco de Dados ===${NC}"
        echo "1) MariaDB"
        echo "2) MySQL"
        echo "3) MongoDB"
        echo "4) PostgreSQL"
        echo "0) Voltar"
        read -rp "Escolha: " opt

        case "$opt" in
            1)
                SELECTED_DB="mariadb"
                log "INFO" "Selecionado: MariaDB"
                install_mariadb || log "ERROR" "Falha na instalação MariaDB"
                config_generic_menu
                ;;
            2)
                SELECTED_DB="mysql"
                log "INFO" "Selecionado: MySQL"
                install_mysql || log "ERROR" "Falha na instalação MySQL"
                config_generic_menu
                ;;
            3)
                SELECTED_DB="mongodb"
                log "INFO" "Selecionado: MongoDB"
                install_mongodb || log "ERROR" "Falha na instalação MongoDB"
                config_generic_menu
                ;;
            4)
                SELECTED_DB="postgresql"
                log "INFO" "Selecionado: PostgreSQL"
                install_postgresql || log "ERROR" "Falha na instalação PostgreSQL"
                config_generic_menu
                ;;
            0) break ;;
            *) echo "Opção inválida" ;;
        esac
    done
}

# ---------- Main menu ----------
main_menu() {
    verifica_basico
    configurar_log

    while true; do
        echo -e "${BOLD}${BLUE}=== Menu Principal DB Config ===${NC}"
        echo "1) Instalar/Configurar Banco de Dados"
        echo "2) Ativar/Desativar Cron de Retenção manualmente"
        echo "3) Imunizar banco/tabela manualmente"
        echo "0) Sair"
        read -rp "Escolha: " opt

        case "$opt" in
            1) select_db_menu ;;
            2)
                echo "1) Ativar cron (daily)"
                echo "2) Desativar cron"
                read -rp "Escolha: " copt
                case "$copt" in
                    1) enable_retention_cron ;;
                    2) disable_retention_cron ;;
                    *) echo "Inválido." ;;
                esac
                ;;
            3)
                read -rp "Nome do item a imunizar (ex: bughunt): " item
                read -rp "DB type (postgresql|mariadb|mysql|mongodb): " dbt
                immunize_item "$item" "$dbt"
                ;;
            0) log "INFO" "Saindo do script."; exit 0 ;;
            *) echo "Opção inválida" ;;
        esac
    done
}

# ---------- Entrypoint ----------
main_menu
