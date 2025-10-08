#!/usr/bin/env bash
set -euo pipefail

# ========================================
# db_config.sh - Database Configuration
# ========================================
# Colors for user interface
BOLD="\033[1m"
GREEN="\033[0;32m"
RED="\033[0;31m"
YELLOW="\033[1;33m"
BLUE="\033[0;34m"
NC="\033[0m"

# Global variables
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
CRED_DIR="$AUTODIR/creds"
MANAGER_TOOL="db_config.sh"
LOGGER_USER="autohunt_logger"
CMD_PACK_MANAGER_INSTALL=""
CMD_PACK_MANAGER_NAME=""
CMD_UPDATE=""

# Ensure directories
MARKER_DIR="$AUTODIR/markers"
mkdir -p "$MARKER_DIR" "$CRED_DIR" 2>/dev/null || {
    echo -e "${RED}Erro: Não foi possível criar diretórios $MARKER_DIR ou $CRED_DIR${NC}"
    exit 1
}
chmod 700 "$CRED_DIR" 2>/dev/null || {
    echo -e "${RED}Erro: Não foi possível definir permissões para $CRED_DIR${NC}"
    exit 1
}

# ---------- Logging utilities ----------
verifica_root() {
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
        mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null || {
            echo -e "${RED}Erro: Não foi possível criar diretório de log $logdir${NC}"
            exit 1
        }
    fi
    touch "$LOG_FILE" 2>/dev/null || {
        echo -e "${RED}Erro: Não foi possível criar arquivo de log $LOG_FILE${NC}"
        exit 1
    }
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
        echo -e "${YELLOW}Aviso: Comando '$1' não encontrado.${NC}"
        return 1
    fi
    return 0
}

random_password() {
    head -c 48 /dev/urandom | tr -dc 'A-Za-z0-9!@%_+' | head -c 24 || echo "autohunt_default_pass"
}

# ---------- Package manager detection ----------
detect_package_manager() {
    local prefix=""
    if [ "$(id -u)" -ne 0 ]; then
        prefix="sudo "
    fi
    if command -v apt >/dev/null 2>&1; then
        CMD_PACK_MANAGER_INSTALL="${prefix}apt install -y"
        CMD_PACK_MANAGER_NAME="apt"
        CMD_UPDATE="${prefix}apt update -y"
    elif command -v pacman >/dev/null 2>&1; then
        CMD_PACK_MANAGER_INSTALL="${prefix}pacman -S --noconfirm"
        CMD_PACK_MANAGER_NAME="pacman"
        CMD_UPDATE="${prefix}pacman -Sy"
    elif command -v dnf >/dev/null 2>&1; then
        CMD_PACK_MANAGER_INSTALL="${prefix}dnf install -y"
        CMD_PACK_MANAGER_NAME="dnf"
        CMD_UPDATE="${prefix}dnf check-update -y"
    elif command -v yum >/dev/null 2>&1; then
        CMD_PACK_MANAGER_INSTALL="${prefix}yum install -y"
        CMD_PACK_MANAGER_NAME="yum"
        CMD_UPDATE="${prefix}yum check-update -y"
    else
        log "ERROR" "Gerenciador de pacotes não detectado. Instale apt/dnf/pacman ou edite o script."
        echo -e "${RED}Erro: Gerenciador de pacotes não detectado.${NC}"
        exit 1
    fi
    log "SUCCESS" "Gerenciador de pacotes detectado: $CMD_PACK_MANAGER_NAME"
}

# ---------- Check database installation ----------
check_db_installation() {
    local db="$1"
    local pkg="$2"
    case "$CMD_PACK_MANAGER_NAME" in
        apt)
            for p in $pkg; do
                dpkg -l "$p" >/dev/null 2>&1 || {
                    log "ERROR" "Pacote $p não instalado."
                    echo -e "${RED}Erro: Pacote $p não instalado. Verifique a instalação de $db.${NC}"
                    return 1
                }
            done
            ;;
        pacman)
            for p in $pkg; do
                pacman -Qs "$p" >/dev/null 2>&1 || {
                    log "ERROR" "Pacote $p não instalado."
                    echo -e "${RED}Erro: Pacote $p não instalado. Verifique a instalação de $db.${NC}"
                    return 1
                }
            done
            ;;
        dnf|yum)
            for p in $pkg; do
                rpm -q "$p" >/dev/null 2>&1 || {
                    log "ERROR" "Pacote $p não instalado."
                    echo -e "${RED}Erro: Pacote $p não instalado. Verifique a instalação de $db.${NC}"
                    return 1
                }
            done
            ;;
        *)
            log "WARN" "Verificação de instalação não suportada para $CMD_PACK_MANAGER_NAME."
            echo -e "${YELLOW}Aviso: Verificação de instalação não suportada para $CMD_PACK_MANAGER_NAME.${NC}"
            return 0
            ;;
    esac
    log "SUCCESS" "Instalação de $db verificada com sucesso."
    return 0
}

# ---------- Check PostgreSQL user ----------
check_postgres_user() {
    if ! id -u postgres >/dev/null 2>&1; then
        log "ERROR" "Usuário 'postgres' não encontrado."
        echo -e "${RED}Erro: Usuário 'postgres' não encontrado. Verifique a instalação do PostgreSQL.${NC}"
        return 1
    fi
    log "INFO" "Usuário 'postgres' encontrado."
    return 0
}

# ---------- Generic DB install ----------
install_db() {
    local db="$1" packages="$2" services="$3"
    log "INFO" "Instalando $db..."
    if [ -z "$CMD_PACK_MANAGER_INSTALL" ]; then
        log "ERROR" "Gerenciador de pacotes não configurado."
        echo -e "${RED}Erro: Gerenciador de pacotes não configurado.${NC}"
        return 1
    fi
    if ! $CMD_UPDATE 2>&1 | tee -a "$LOG_FILE"; then
        echo -e "${RED}Erro: Falha ao atualizar repositórios com $CMD_UPDATE${NC}"
        return 1
    fi
    if ! $CMD_PACK_MANAGER_INSTALL $packages 2>&1 | tee -a "$LOG_FILE"; then
        echo -e "${RED}Erro: Falha ao instalar $db${NC}"
        return 1
    fi
    if ! check_db_installation "$db" "$packages"; then
        return 1
    fi
    if [ -n "$SERVICE_CMD" ]; then
        for svc in $services; do
            if ! $SERVICE_CMD enable "$svc" 2>&1 | tee -a "$LOG_FILE"; then
                echo -e "${YELLOW}Aviso: Falha ao habilitar o serviço $svc${NC}"
            fi
            if ! $SERVICE_CMD start "$svc" 2>&1 | tee -a "$LOG_FILE"; then
                echo -e "${YELLOW}Aviso: Falha ao iniciar o serviço $svc${NC}"
            fi
        done
    else
        echo -e "${YELLOW}Aviso: Nenhum comando de serviço (systemctl/service) encontrado${NC}"
    fi
    log "SUCCESS" "$db instalado e iniciado."
}

# ---------- Create DB and marker ----------
create_bughunt_db() {
    local db="$1"
    log "INFO" "Criando banco bughunt no $db (se não existir) e registrando marker."
    case "$db" in
        postgresql)
            if check_command sqlite3; then
                DB_PATH="bug_hunt.db"
                sqlite3 "$DB_PATH" || {
                    echo -e "${YELLOW}Aviso: Falha ao criar banco bughunt no SQLite${NC}"
                }
                log "INFO" "Banco SQLite criado em $DB_PATH."

            else
                log "ERROR" "sqlite3 não encontrado."
                echo -e "${RED}Erro: sqlite3 não encontrado.${NC}"
                return 1
            fi
            ;;

        postgresql)
            if check_command psql && check_postgres_user; then
                if ! sudo -u postgres psql -c "CREATE DATABASE bughunt;" 2>&1 | tee -a "$LOG_FILE"; then
                    echo -e "${YELLOW}Aviso: Falha ao criar banco bughunt no PostgreSQL${NC}"
                fi
            else
                log "ERROR" "psql ou usuário postgres não encontrado."
                echo -e "${RED}Erro: psql ou usuário postgres não encontrado.${NC}"
                return 1
            fi
            ;;
        mariadb|mysql)
            if check_command mysql; then
                if ! mysql -u root -e "CREATE DATABASE IF NOT EXISTS bughunt;" 2>&1 | tee -a "$LOG_FILE"; then
                    log "WARN" "Falha ao criar DB via mysql client."
                    echo -e "${YELLOW}Aviso: Falha ao criar banco bughunt no $db${NC}"
                fi
            else
                log "ERROR" "mysql client não encontrado."
                echo -e "${RED}Erro: mysql client não encontrado.${NC}"
                return 1
            fi
            ;;
        mongodb)
            local mongo_cmd
            if check_command mongosh; then
                mongo_cmd="mongosh"
            elif check_command mongo; then
                mongo_cmd="mongo"
            else
                log "ERROR" "Nem mongosh nem mongo encontrados."
                echo -e "${RED}Erro: Nem mongosh nem mongo encontrados.${NC}"
                return 1
            fi
            if ! $mongo_cmd --quiet --eval 'db.getSiblingDB("bughunt").createCollection("init")' 2>&1 | tee -a "$LOG_FILE"; then
                log "WARN" "Falha ao criar banco bughunt no MongoDB."
                echo -e "${YELLOW}Aviso: Falha ao criar banco bughunt no MongoDB${NC}"
            fi
            ;;
        *)
            log "ERROR" "DB desconhecido: $db"
            echo -e "${RED}Erro: Banco de dados desconhecido: $db${NC}"
            return 1
            ;;
    esac
    local marker_file="$MARKER_DIR/bughunt.$db.marker"
    date +%s > "$marker_file" 2>&1 | tee -a "$LOG_FILE" || {
        echo -e "${RED}Erro: Não foi possível criar marcador $marker_file${NC}"
        return 1
    }
    chmod 600 "$marker_file" 2>&1 | tee -a "$LOG_FILE" || {
        echo -e "${RED}Erro: Não foi possível definir permissões para $marker_file${NC}"
        return 1
    }
}

# ---------- Create logger user ----------
create_logger_user() {
    local db="$1"
    local pass
    pass="$(random_password)"
    mkdir -p "$CRED_DIR" 2>/dev/null || {
        echo -e "${RED}Erro: Não foi possível criar diretório $CRED_DIR${NC}"
        return 1
    }
    case "$db" in
        postgresql)
            if check_command psql && check_postgres_user; then




                if ! sudo -u postgres psql -c "DO \$\$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${LOGGER_USER}') THEN CREATE ROLE ${LOGGER_USER} LOGIN PASSWORD '${pass}'; END IF; END\$\$;" 2>&1 | tee -a "$LOG_FILE"; then
                    echo -e "${YELLOW}Aviso: Falha ao criar usuário logger no PostgreSQL${NC}"
                fi
                if ! sudo -u postgres psql -c "GRANT CONNECT ON DATABASE bughunt TO ${LOGGER_USER};" 2>&1 | tee -a "$LOG_FILE"; then
                    echo -e "${YELLOW}Aviso: Falha ao conceder permissão CONNECT no PostgreSQL${NC}"
                fi
                if ! sudo -u postgres psql -d bughunt -c "GRANT INSERT ON ALL TABLES IN SCHEMA public TO ${LOGGER_USER};" 2>&1 | tee -a "$LOG_FILE"; then
                    echo -e "${YELLOW}Aviso: Falha ao conceder permissão INSERT no PostgreSQL${NC}"
                fi
            else
                log "ERROR" "psql ou usuário postgres não encontrado; usuário logger não criado."
                echo -e "${RED}Erro: psql ou usuário postgres não encontrado.${NC}"
                return 1
            fi
            ;;
        mariadb|mysql)
            if check_command mysql; then
                if ! mysql -u root -e "CREATE USER IF NOT EXISTS '${LOGGER_USER}'@'localhost' IDENTIFIED BY '${pass}'; GRANT INSERT ON bughunt.* TO '${LOGGER_USER}'@'localhost'; FLUSH PRIVILEGES;" 2>&1 | tee -a "$LOG_FILE"; then
                    log "WARN" "Falha ao criar usuário logger no $db."
                    echo -e "${YELLOW}Aviso: Falha ao criar usuário logger no $db${NC}"
                fi
            else
                log "ERROR" "mysql client não encontrado; usuário logger não criado."
                echo -e "${RED}Erro: mysql client não encontrado.${NC}"
                return 1
            fi
            ;;
        mongodb)
            local mongo_cmd
            if check_command mongosh; then
                mongo_cmd="mongosh"
            elif check_command mongo; then
                mongo_cmd="mongo"
            else
                log "ERROR" "Nem mongosh nem mongo encontrados; usuário logger não criado."
                echo -e "${RED}Erro: Nem mongosh nem mongo encontrados.${NC}"
                return 1
            fi
            if ! $mongo_cmd bughunt --quiet --eval "db.createUser({user: '${LOGGER_USER}', pwd: '${pass}', roles: [{role: 'readWrite', db: 'bughunt'}]})" 2>&1 | tee -a "$LOG_FILE"; then
                log "WARN" "Falha ao criar usuário logger no MongoDB."
                echo -e "${YELLOW}Aviso: Falha ao criar usuário logger no MongoDB${NC}"
            fi
            ;;
        *)
            log "ERROR" "DB desconhecido para criar logger user: $db"
            echo -e "${RED}Erro: Banco de dados desconhecido: $db${NC}"
            return 1
            ;;
    esac
    local cred_file="$CRED_DIR/${LOGGER_USER}_${db}.txt"
    {
        echo "db=$db"
        echo "user=${LOGGER_USER}"
        echo "pass=${pass}"
        echo "created=$(date --iso-8601=seconds 2>/dev/null || date)"
    } > "$cred_file" 2>&1 | tee -a "$LOG_FILE" || {
        echo -e "${RED}Erro: Não foi possível criar arquivo de credenciais $cred_file${NC}"
        return 1
    }
    chmod 600 "$cred_file" 2>&1 | tee -a "$LOG_FILE" || {
        echo -e "${RED}Erro: Não foi possível definir permissões para $cred_file${NC}"
        return 1
    }
    log "INFO" "Usuário logger criado para $db. Credenciais em: $cred_file"
}

# ---------- Metasploit integration ----------
ensure_metasploit_db() {
    local db="$1"
    local msf_pass
    read -rp "Digite a senha para o usuário msf (deixe em branco para gerar aleatoriamente): " msf_pass
    if [ -z "$msf_pass" ]; then
        msf_pass="$(random_password)"
        echo -e "${BLUE}Senha gerada para usuário msf: $msf_pass${NC}"
    fi
    case "$db" in
        postgresql)
            if check_command psql && check_postgres_user; then
                if ! sudo -u postgres psql -c "DO \$\$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'msf') THEN CREATE USER msf WITH PASSWORD '$msf_pass'; END IF; END\$\$;" 2>&1 | tee -a "$LOG_FILE"; then
                    echo -e "${YELLOW}Aviso: Falha ao criar usuário msf no PostgreSQL${NC}"
                fi
                if ! sudo -u postgres psql -c "CREATE DATABASE IF NOT EXISTS msf;" 2>&1 | tee -a "$LOG_FILE"; then
                    echo -e "${YELLOW}Aviso: Falha ao criar banco msf no PostgreSQL${NC}"
                fi
                log "INFO" "Metasploit DB/user ensured on PostgreSQL."
            else
                log "ERROR" "psql ou usuário postgres não encontrado."
                echo -e "${RED}Erro: psql ou usuário postgres não encontrado.${NC}"
                return 1
            fi
            ;;
        mariadb|mysql)
            if check_command mysql; then
                if ! mysql -u root -e "CREATE DATABASE IF NOT EXISTS msf; CREATE USER IF NOT EXISTS 'msf'@'localhost' IDENTIFIED BY '$msf_pass'; GRANT ALL PRIVILEGES ON msf.* TO 'msf'@'localhost'; FLUSH PRIVILEGES;" 2>&1 | tee -a "$LOG_FILE"; then
                    log "WARN" "Falha ao configurar banco ou usuário msf no $db."
                    echo -e "${YELLOW}Aviso: Falha ao configurar banco ou usuário msf no $db${NC}"
                fi
                log "INFO" "Metasploit DB/user ensured on MySQL/MariaDB."
            else
                log "ERROR" "mysql client não encontrado."
                echo -e "${RED}Erro: mysql client não encontrado.${NC}"
                return 1
            fi
            ;;
        *)
            log "WARN" "Metasploit DB not applicable for $db"
            echo -e "${YELLOW}Aviso: Metasploit não é aplicável para $db${NC}"
            ;;
    esac
}

connect_metasploit() {
    local msf_dir="$HOME/.msf4"
    mkdir -p "$msf_dir" 2>&1 | tee -a "$LOG_FILE" || {
        echo -e "${RED}Erro: Não foi possível criar diretório $msf_dir${NC}"
        return 1
    }
    case "$SELECTED_DB" in
        mariadb|mysql)
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
        postgresql)
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
        mongodb)
            log "WARN" "Metasploit não suporta MongoDB."
            echo -e "${YELLOW}Aviso: Metasploit não suporta MongoDB.${NC}"
            return
            ;;
        *)
            log "ERROR" "Tipo de DB desconhecido para Metasploit: $SELECTED_DB"
            echo -e "${RED}Erro: Tipo de DB desconhecido para Metasploit: $SELECTED_DB${NC}"
            return
            ;;
    esac
    chmod 600 "$msf_dir/database.yml" 2>&1 | tee -a "$LOG_FILE" || {
        echo -e "${RED}Erro: Não foi possível definir permissões para $msf_dir/database.yml${NC}"
        return 1
    }
    log "SUCCESS" "Arquivo ~/.msf4/database.yml criado para $SELECTED_DB."
}

# ---------- Generic configuration menu ----------
config_generic_menu() {
    while true; do
        echo -e "${BOLD}${GREEN}=== Configuração do $SELECTED_DB ===${NC}"
        echo "1) Criar serviço de inicialização do $SELECTED_DB"
        echo "2) Criar banco 'bughunt' e registrar marker"
        echo "3) Criar usuário logger (autohunt_logger)"
        echo "4) Conectar automaticamente ao Metasploit"
        echo "0) Voltar"
        read -rp "Escolha: " opt
        case "$opt" in
            1)
                case "$SELECTED_DB" in
                    mariadb)
                        if [ -n "$SERVICE_CMD" ]; then
                            if ! $SERVICE_CMD enable mariadb 2>&1 | tee -a "$LOG_FILE"; then
                                echo -e "${YELLOW}Aviso: Falha ao habilitar o serviço MariaDB${NC}"
                            fi
                        else
                            echo -e "${YELLOW}Aviso: Nenhum comando de serviço encontrado${NC}"
                        fi
                        ;;
                    mysql)
                        if [ -n "$SERVICE_CMD" ]; then
                            if ! $SERVICE_CMD enable mysql 2>&1 | tee -a "$LOG_FILE"; then
                                echo -e "${YELLOW}Aviso: Falha ao habilitar o serviço MySQL${NC}"
                            fi
                        else
                            echo -e "${YELLOW}Aviso: Nenhum comando de serviço encontrado${NC}"
                        fi
                        ;;
                    mongodb)
                        if [ -n "$SERVICE_CMD" ]; then
                            if ! $SERVICE_CMD enable mongod 2>&1 | tee -a "$LOG_FILE" && ! $SERVICE_CMD enable mongodb 2>&1 | tee -a "$LOG_FILE"; then
                                echo -e "${YELLOW}Aviso: Falha ao habilitar o serviço MongoDB${NC}"
                            fi
                        else
                            echo -e "${YELLOW}Aviso: Nenhum comando de serviço encontrado${NC}"
                        fi
                        ;;
                    postgresql)
                        if [ -n "$SERVICE_CMD" ]; then
                            if ! $SERVICE_CMD enable postgresql 2>&1 | tee -a "$LOG_FILE"; then
                                echo -e "${YELLOW}Aviso: Falha ao habilitar o serviço PostgreSQL${NC}"
                            fi
                        else
                            echo -e "${YELLOW}Aviso: Nenhum comando de serviço encontrado${NC}"
                        fi
                        ;;
                esac
                log "INFO" "$SELECTED_DB configurado para iniciar com o sistema."
                ;;
            2)
                create_bughunt_db "$SELECTED_DB"
                ;;
            3)
                create_logger_user "$SELECTED_DB"
                ;;
            4)
                ensure_metasploit_db "$SELECTED_DB"
                connect_metasploit
                ;;
            0) break ;;
            *) echo "Opção inválida" ;;
        esac
        pause
    done
}

# ---------- Select and installation menu ----------
select_db_menu() {
    while true; do
        echo -e "${BOLD}${GREEN}=== Seleção de Banco de Dados ===${NC}"
        echo "1) MariaDB"
        echo "2) MySQL"
        echo "3) MongoDB"
        echo "4) PostgreSQL"
		echo "5) SQLite"
        echo "0) Voltar"
        read -rp "Escolha: " opt
        case "$opt" in
            1)
                SELECTED_DB="mariadb"
                log "INFO" "Selecionado: MariaDB"
                install_db "MariaDB" "mariadb-server mariadb-client" "mariadb" || {
                    log "ERROR" "Falha na instalação MariaDB"
                    echo -e "${RED}Erro: Falha na instalação MariaDB${NC}"
                }
                config_generic_menu
                ;;
            2)
                SELECTED_DB="mysql"
                log "INFO" "Selecionado: MySQL"
                install_db "MySQL" "mysql-server mysql-client" "mysql" || {
                    log "ERROR" "Falha na instalação MySQL"
                    echo -e "${RED}Erro: Falha na instalação MySQL${NC}"
                }
                config_generic_menu
                ;;
            3)
                SELECTED_DB="mongodb"
                log "INFO" "Selecionado: MongoDB"
                install_db "MongoDB" "mongodb mongodb-org" "mongod mongodb" || {
                    log "ERROR" "Falha na instalação MongoDB"
                    echo -e "${RED}Erro: Falha na instalação MongoDB${NC}"
                }
                config_generic_menu
                ;;
            4)
                SELECTED_DB="sqlite"
                log "INFO" "Selecionado: SQLite"
                install_db "SQLite" "sqlite3" "sqlite" || {
                    log "ERROR" "Falha na instalação SQLite"
                    echo -e "${RED}Erro: Falha na instalação do SQLite${NC}"
                }
                config_generic_menu
                ;;



            5)
                SELECTED_DB="postgresql"
                log "INFO" "Selecionado: PostgreSQL"
                install_db "PostgreSQL" "postgresql postgresql-contrib" "postgresql" || {
                    log "ERROR" "Falha na instalação PostgreSQL"
                    echo -e "${RED}Erro: Falha na instalação PostgreSQL${NC}"
                }
                config_generic_menu
                ;;



            0) break ;;
            *) echo "Opção inválida" ;;
        esac
    done
}

# ---------- Main menu ----------
main() {
    verifica_root
    configurar_log
    detect_package_manager

    echo -e "${BLUE}Iniciando configuração de banco de dados...${NC}"
    select_db_menu

    log "INFO" "Saindo do script de configuração de DB."
    echo -e "\n${GREEN}Configuração do banco de dados concluída.${NC}"
    echo -e "Para gerenciar o ambiente (serviços, retenção, etc.), use o script 'config_enviroment.sh'."
    exit 0
}

# ---------- Entrypoint ----------
main
