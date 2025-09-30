#!/usr/bin/env bash
set -euo pipefail

# =============================
# Configurações
# =============================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

DB_NAME="autohunt_db"
DB_USER="autohunt_user"
DB_PASSWORD="autohunt_pass"
DATA_DIR="/opt/autohunt_data"
RETENTION_DAYS=30
LOG_FILE="/var/log/db_config.log"

# =============================
# Logging
# =============================
log() {
    local level="$1"; shift
    local msg="$*"
    echo -e "[$(date '+%F %T')] ${level}: $msg" | tee -a "$LOG_FILE"
}

# =============================
# Check root
# =============================
check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        echo -e "${RED}Este script precisa ser executado como root!${NC}"
        exit 1
    fi
}

# =============================
# Detect package manager
# =============================
detect_package_manager() {
    if command -v apt >/dev/null 2>&1; then
        PKG_INSTALL="apt install -y"
        SERVICE_CMD="systemctl"
    elif command -v dnf >/dev/null 2>&1; then
        PKG_INSTALL="dnf install -y"
        SERVICE_CMD="systemctl"
    elif command -v yum >/dev/null 2>&1; then
        PKG_INSTALL="yum install -y"
        SERVICE_CMD="systemctl"
    else
        log "ERROR" "Gerenciador de pacotes não detectado. Instale apt, dnf ou yum."
        exit 1
    fi
    log "INFO" "Gerenciador de pacotes detectado"
}

# =============================
# Instalar PostgreSQL
# =============================
install_postgres() {
    log "INFO" "Instalando PostgreSQL..."
    $PKG_INSTALL postgresql postgresql-contrib
    log "INFO" "Habilitando e iniciando serviço PostgreSQL..."
    $SERVICE_CMD enable postgresql
    $SERVICE_CMD start postgresql
}

# =============================
# Configurar usuário e banco
# =============================
setup_db() {
    log "INFO" "Criando usuário e banco de dados..."
    sudo -u postgres psql <<EOF
DO
\$do\$
BEGIN
   IF NOT EXISTS (SELECT FROM pg_catalog.pg_user WHERE usename = '${DB_USER}') THEN
      CREATE USER ${DB_USER} WITH PASSWORD '${DB_PASSWORD}';
   END IF;
END
\$do\$;
CREATE DATABASE ${DB_NAME} OWNER ${DB_USER};
EOF
    log "INFO" "Usuário e banco criados com sucesso"
}

# =============================
# Configurar diretório para dados sujos
# =============================
setup_data_dir() {
    read -rp "Informe o diretório para armazenar dados sujos [default: ${DATA_DIR}]: " input_dir
    DATA_DIR="${input_dir:-$DATA_DIR}"
    mkdir -p "$DATA_DIR"
    chown -R postgres:postgres "$DATA_DIR"
    log "INFO" "Diretório de dados configurado: $DATA_DIR"
}

# =============================
# Política de retenção
# =============================
setup_retention_policy() {
    read -rp "Informe a política de retenção em dias [default: ${RETENTION_DAYS}]: " days
    RETENTION_DAYS="${days:-$RETENTION_DAYS}"
    log "INFO" "Política de retenção configurada: ${RETENTION_DAYS} dias"
}

# =============================
# Menu de criação de tabelas
# =============================
table_menu() {
    while true; do
        echo -e "${BOLD}${GREEN}=== Menu de Banco de Dados ===${NC}"
        echo "1) Criar tabela"
        echo "2) Adicionar coluna a tabela existente"
        echo "3) Adicionar chave primária"
        echo "4) Listar tabelas"
        echo "5) Voltar ao menu principal"
        read -rp "Escolha uma opção: " opt

        case "$opt" in
            1)
                read -rp "Nome da nova tabela: " table
                sudo -u postgres psql -d "$DB_NAME" -c "CREATE TABLE IF NOT EXISTS $table ();"
                log "INFO" "Tabela $table criada"
                ;;
            2)
                read -rp "Nome da tabela: " table
                read -rp "Nome da coluna: " col
                read -rp "Tipo de dado (ex: TEXT, INTEGER): " type
                sudo -u postgres psql -d "$DB_NAME" -c "ALTER TABLE $table ADD COLUMN $col $type;"
                log "INFO" "Coluna $col adicionada à tabela $table"
                ;;
            3)
                read -rp "Nome da tabela: " table
                read -rp "Coluna da chave primária: " col
                sudo -u postgres psql -d "$DB_NAME" -c "ALTER TABLE $table ADD PRIMARY KEY ($col);"
                log "INFO" "Chave primária adicionada à tabela $table"
                ;;
            4)
                sudo -u postgres psql -d "$DB_NAME" -c "\dt"
                ;;
            5) break ;;
            *) echo "Opção inválida" ;;
        esac
    done
}

# =============================
# Conexão com próximo script
# =============================
connect_next_script() {
    read -rp "Deseja executar o próximo script de tratamento de dados? [s/N]: " resp
    if [[ "$resp" =~ ^[sS]$ ]]; then
        read -rp "Informe o caminho do script: " script_path
        if [[ -f "$script_path" && -x "$script_path" ]]; then
            log "INFO" "Executando script $script_path..."
            "$script_path" "$DATA_DIR"
        else
            log "ERROR" "Script não encontrado ou não executável"
        fi
    fi
}

# =============================
# Menu principal
# =============================
main_menu() {
    while true; do
        echo -e "${BOLD}${GREEN}=== Configuração do Banco de Dados PostgreSQL ===${NC}"
        echo "1) Instalar PostgreSQL e configurar serviço"
        echo "2) Criar usuário e banco de dados"
        echo "3) Configurar diretório de dados sujos"
        echo "4) Configurar política de retenção"
        echo "5) Menu de tabelas e colunas"
        echo "6) Conectar próximo script de tratamento"
        echo "0) Sair"
        read -rp "Escolha uma opção: " choice

        case "$choice" in
            1) install_postgres ;;
            2) setup_db ;;
            3) setup_data_dir ;;
            4) setup_retention_policy ;;
            5) table_menu ;;
            6) connect_next_script ;;
            0) log "INFO" "Saindo..."; exit 0 ;;
            *) echo "Opção inválida" ;;
        esac
    done
}

# =============================
# Main
# =============================
check_root
detect_package_manager
mkdir -p "$(dirname "$LOG_FILE")"
touch "$LOG_FILE"
main_menu
