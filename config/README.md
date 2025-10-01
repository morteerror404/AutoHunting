# db_config.sh - Database Configuration Script

`db_config.sh` é um script Bash para instalação, configuração e gerenciamento de bancos de dados (PostgreSQL, MariaDB, MySQL, MongoDB) com integração ao Metasploit, criação de usuários, política de retenção de dados e configuração de cron jobs. Ele oferece um menu interativo para facilitar a administração de sistemas de bancos de dados em ambientes Linux.

## Funcionalidades

- **Instalação de Bancos de Dados**: Suporta PostgreSQL, MariaDB, MySQL e MongoDB, com verificação de instalação bem-sucedida.
- **Criação de Banco e Usuário**: Cria o banco `bughunt` e o usuário `autohunt_logger` com permissões específicas.
- **Integração com Metasploit**: Configura um banco e usuário (`msf`) para uso com o Metasploit, com senha personalizável.
- **Política de Retenção**: Implementa um sistema de retenção baseado em marcadores de tempo, com opção de imunizar bancos contra exclusão.
- **Cron Job Personalizável**: Permite configurar a exclusão automática de bancos com base em um período de retenção, com horário e frequência ajustáveis (diária, semanal, mensal).
- **Logs Detalhados**: Registra todas as operações em um arquivo de log (`/var/log/db_config.log` ou `~/.db_config.log`).
- **Tratamento de Erros**: Exibe mensagens claras no terminal para falhas, facilitando a depuração.

## Pré-requisitos

- **Sistema Operacional**: Distribuições Linux com gerenciador de pacotes `apt`, `pacman`, `dnf` ou `yum`.
- **Permissões**: O script deve ser executado como root ou com `sudo`.
- **Dependências**:
  - Ferramentas de linha de comando: `bash`, `tee`, `systemctl` ou `service`.
  - Clientes de banco de dados: `psql` (PostgreSQL), `mysql` (MariaDB/MySQL), `mongosh` ou `mongo` (MongoDB).
- **Acesso à Internet**: Necessário para instalar pacotes, a menos que os repositórios estejam disponíveis localmente.

## Instalação

1. **Clone ou Baixe o Script**:
   ```bash
   git clone https://github.com/morteerror404/AutoHunting/blob/main/config/db_config.sh
   ```
   ```bash
   wget <URL_do_script>/db_config.sh
   ```

2. **Torne o Script Executável**:
   ```bash
   chmod +x db_config.sh
   ```

3. **Execute o Script**:
   ```bash
   sudo ./db_config.sh
   ```

## Uso

O script apresenta um menu interativo com as seguintes opções:

1. **Instalar/Configurar Banco de Dados**:
   - Escolha entre MariaDB, MySQL, MongoDB ou PostgreSQL.
   - Instala o banco selecionado, configura serviços de inicialização, cria o banco `bughunt`, usuário `autohunt_logger` e integra com Metasploit.
   - Permite configurar a política de retenção e imunizar bancos/tabelas.

2. **Ativar/Desativar Cron de Retenção**:
   - Ativa um cron job para excluir bancos com base no período de retenção (padrão: 30 dias).
   - Permite personalizar horário (HH:MM) e frequência (diária, semanal, mensal).
   - Desativa o cron job, se necessário.

3. **Imunizar Banco/Tabela**:
   - Protege bancos/tabelas específicos contra exclusão automática pelo sistema de retenção.

4. **Sair**:
   - Encerra a execução do script.

### Exemplo de Execução
```bash
sudo ./db_config.sh
```
- Escolha `1` para instalar e configurar um banco.
- Selecione, por exemplo, `4` para PostgreSQL.
- No submenu, escolha `2` para criar o banco `bughunt` ou `6` para configurar um cron job com horário personalizado.

## Estrutura de Arquivos

- **Logs**: `/var/log/db_config.log` (ou `~/.db_config.log` para usuários não-root).
- **Marcadores de Retenção**: `/var/lib/autohunt/markers/`.
- **Credenciais**: `/var/lib/autohunt/creds/` (permissões restritas a 600).
- **Script de Retenção**: `/usr/local/bin/autohunt_retention_cleanup.sh`.
- **Cron Job**: `/etc/cron.d/autohunt_retention`.
- **Aviso de Terminal**: `/etc/profile.d/autohunt_retention_warning.sh`.

## Configuração Avançada

### Personalizar Período de Retenção
No submenu de configuração do banco, escolha a opção `5` para definir o número de dias de retenção (`RETENTION_DAYS`).

### Configurar Cron Job
Na opção `6` do submenu, especifique:
- **Horário**: Formato HH:MM (ex.: `14:30`).
- **Frequência**: Diária, semanal (domingo) ou mensal (dia 1).

### Imunizar Bancos
Na opção `8` do submenu ou `3` no menu principal, forneça o nome do banco/tabela (ex.: `bughunt`) e o tipo de banco (ex.: `postgresql`) para protegê-lo contra exclusão.

## Segurança

- **Credenciais**: O usuário `autohunt_logger` tem uma senha gerada aleatoriamente, armazenada em `/var/lib/autohunt/creds/` com permissões restritas.
- **Metasploit**: O usuário `msf` permite senha personalizada (ou gerada aleatoriamente).
- **Aviso**: Credenciais em texto puro são um risco. Considere usar um gerenciador de segredos para maior segurança.

## Limitações

- Suporte limitado a gerenciadores de pacotes (`apt`, `pacman`, `dnf`, `yum`). Outros, como `zypper` ou `apk`, requerem adaptação.
- O MongoDB suporta tanto `mongosh` quanto `mongo`, mas sistemas muito antigos podem exigir ajustes.
- A integração com Metasploit não suporta MongoDB.
- Não há suporte nativo para backup automático antes da exclusão de bancos pela política de retenção.