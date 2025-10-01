# AutoHunting - README

## Descrição
O `install.sh` é um script Bash projetado para automatizar a instalação de ferramentas de segurança cibernética e reconciliação (recon) em sistemas Linux baseados em Arch (usando `pacman`), Debian (apt), Fedora (dnf/yum) ou outros sistemas compatíveis. Ele suporta instalações via gerenciadores de pacotes do sistema, Go, Python (pip), e clonagem de repositórios Git. O script é interativo, permitindo ao usuário selecionar categorias de ferramentas ou instalar todas de uma vez, com suporte a instalações paralelas para maior eficiência.

## Funcionalidades
- **Gerenciamento de Pacotes**: Detecta automaticamente o gerenciador de pacotes (`pacman`, `apt`, `dnf`, ou `yum`) e instala ferramentas compatíveis.
- **Métodos de Instalação**:
  - **Gerenciador de Pacotes**: Usa o gerenciador do sistema para instalar ferramentas disponíveis (e.g., `curl`, `nmap`).
  - **Go**: Instala ferramentas Go-based com `go install` (e.g., `subfinder`, `nuclei`).
  - **Pip**: Instala ferramentas Python-based com `pip3` (e.g., `wafw00f`, `linkfinder`).
  - **Git**: Clona repositórios Git para ferramentas sem pacotes pré-compilados (e.g., `katana`, `reconftw`).
- **Categorias de Ferramentas**:
  - **Base**: Ferramentas essenciais (`curl`, `jq`, `git`, `nmap`, `go`, `python`).
  - **Recon**: Ferramentas de reconciliação (`subfinder`, `amass`, `katana`, etc.).
  - **Scanners**: Ferramentas de varredura (`nuclei`, `ffuf`, `nikto`, etc.).
  - **Web**: Ferramentas para análise web (`gopherus`, `lfisuite`, etc.).
  - **Aux**: Ferramentas auxiliares (`kiterunner`, `eyewitness`, etc.).
  - **Misc**: Ferramentas diversas (`linkfinder`, `metasploit`, etc.).
- **Paralelismo**: Instala até 4 pacotes simultaneamente (`MAX_JOBS=4`) para maior rapidez.
- **Modo Simulação**: Suporta execução em modo *dry-run* (`DRY_RUN=1`) para testar sem alterações.
- **Logging**: Gera logs detalhados em `/var/log/autohunting_install.log`.
- **Resumo de Instalação**: Exibe um resumo das ferramentas instaladas, incluindo métodos usados.

## Estrutura do Script

### Configurações
- **Cores**: Usa códigos ANSI para mensagens coloridas no terminal.
- **Variáveis**:
  - `LOG_FILE`: `/var/log/autohunting_install.log` (log de saída).
  - `TOOLS_PREFIX`: `/opt/autohunting` (diretório base para instalações).
  - `GIT_INSTALL_DIR`: Diretório para clonagem Git (definido pelo usuário ou padrão).
  - `MAX_JOBS`: 4 (máximo de instalações paralelas).
  - `GIT_REPOS`: Array associativo mapeando ferramentas a URLs de repositórios Git.

### Funções Principais
- **`log_message`**: Registra mensagens com nível (INFO, ERROR, DEBUG, etc.) no terminal e log.
- **`detect_package_manager`**: Identifica o gerenciador de pacotes do sistema.
- **`_exec_install`**: Executa instalações via gerenciador de pacotes.
- **`get_install_cmd`**: Define comandos especiais (Go, pip, ou Git) para ferramentas específicas.
- **`pkg_binary_candidates`**: Mapeia ferramentas a seus binários para verificação.
- **`is_installed`**: Verifica se uma ferramenta está instalada.
- **`install_one`**: Instala uma ferramenta, seguindo esta ordem:
  1. Verifica se já está instalada (`is_installed`).
  2. Tenta método especial (Go/pip via `get_install_cmd`).
  3. Usa gerenciador de pacotes (`_exec_install`).
  4. Clona repositório Git como fallback (`GIT_REPOS`).
- **`install_array`**: Instala múltiplas ferramentas em paralelo.
- **`install_category_by_name`**: Instala ferramentas de uma categoria específica.
- **`show_modules`**: Lista todas as categorias e ferramentas.
- **`show_install_summary`**: Exibe resumo das instalações.
- **`verifica_basico`**: Garante privilégios de root e dependências (`git`, `tee`).
- **`configurar_log`**: Configura redirecionamento de logs.
- **`print_menu`**: Exibe menu interativo.
- **`main`**: Função principal que gerencia o fluxo do script.

### Fluxo de Instalação
Para cada ferramenta, o script tenta:
1. **Verificar Instalação**: Se já instalada, pula.
2. **Método Especial**: Usa Go (`go install`), pip (`pip3 install`), ou Git clone para ferramentas específicas.
3. **Gerenciador de Pacotes**: Tenta instalar com `pacman`, `apt`, etc.
4. **Git Clone**: Clona o repositório se outros métodos falharem.

## Pré-requisitos
- Sistema Linux com `pacman`, `apt`, `dnf`, ou `yum`.
- Privilégios de root (use `sudo`).
- Dependências: `git`, `go`, `python3-pip`, `tee`.
- Conexão com a internet para downloads e clonagem Git.

## Instalação e Uso
1. **Configurar Permissões**:
   Defina permissões seguras para os arquivos do script:
   ```bash
   sudo chmod 600 ./install.sh ; chmod 600 ./config/db_config.sh
   ```

2. **Instalar Dependências**:
   Para sistemas baseados em Arch:
   ```bash
   sudo pacman -Syu
   sudo pacman -S git go python-pip coreutils
   ```

3. **Executar o Script**:
   ```bash
   sudo ./install.sh
   ```
   - O script exibirá um menu interativo.
   - Selecione opções (e.g., `2` para base, `3` para recon) ou múltiplas opções separadas por vírgula (e.g., `2,3`).
   - Para sair, selecione `0`.

4. **Modo Simulação**:
   Para testar sem instalar:
   ```bash
   DRY_RUN=1 sudo ./install.sh
   ```

5. **Verificar Logs**:
   Consulte o log para detalhes ou erros:
   ```bash
   cat /var/log/autohunting_install.log
   ```

## Exemplo de Uso
```bash
$ sudo ./install.sh
=== Installer ===
Escolha categorias (múltiplas separadas por vírgula):
 1) Todas
 2) Base
 3) Recon
 4) Scanners
 5) Web
 6) Aux
 7) Misc
 8) Mostrar módulos
 9) Verificar instalação
 0) Sair
Opção(s): 3
[2025-09-30 23:18:00] SUCCESS: Gerenciador de pacotes detectado: pacman
[2025-09-30 23:18:01] INFO: [1/10] Instalando: subfinder
[2025-09-30 23:18:02] INFO: [4/10] Instalando: katana
...
=== Resumo das instalações ===
Tools installed by pacman:
  curl
Tools installed by git clone:
  katana
    URL = https://github.com/projectdiscovery/katana
    Dir = /opt/autohunting/katana
Instalação concluída!
```

## Depuração
- **Verificar Logs**: Consulte `/var/log/autohunting_install.log` para erros.
- **Debugging**:
  ```bash
  bash -x ./install.sh
  ```
- **Testar Ferramentas**: Após a instalação, verifique ferramentas Git:
  ```bash
  ls -l /opt/autohunting
  ```

## Notas
- **Paralelismo**: O script usa até 4 processos simultâneos. Reduza `MAX_JOBS` em sistemas com poucos recursos.
- **Git**: Ferramentas como `katana` e `reconftw` são clonadas para `/opt/autohunting` ou um diretório especificado.
- **Permissões**: O comando `chmod 600` garante que apenas o proprietário (root) acesse os scripts.
- **Erros**: Falhas de instalação (e.g., GitHub inacessível) são registradas no log.

## Limitações
- Algumas ferramentas (e.g., `reconftw`, `metasploit`) podem exigir configuração manual após clonagem.
- Requer conexão com a internet para Git e downloads de pacotes.
- A verificação de instalação (`is_installed`) pode falhar para ferramentas sem binários padrão.