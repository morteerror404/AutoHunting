# Configuração do AutoHunting

Este diretório contém configurações essenciais para o projeto AutoHunting, incluindo arquivos JSON que definem parâmetros para a pipeline de coleta, varredura, limpeza e armazenamento de dados de bug bounty.

## Estrutura do Diretório
```
config/
├── README.md
└── json/
    ├── cleaner-templates.json
    ├── commands.json
    ├── order-templates.json
    ├── db_info.json
    ├── env.json
    └── tokens.json
```

## Arquivos JSON

### 1. `cleaner-templates.json`
Define padrões de limpeza (regex e campos) para processar resultados brutos de ferramentas de varredura (`nmap` e `ffuf`). Usado por `data/cleaner/cleaner.go`.

- **Formato**:
  ```json
  {
      "nmap": {
          "open_ports": {
              "regex": "^(\\d+)/tcp\\s+open\\s+[^\\s]+",
              "fields": ["port"]
          }
      },
      "ffuf": {
          "endpoints": {
              "regex": "^(https?://[^\\s]+)$",
              "fields": ["url"]
          }
      }
  }
  ```
- **Uso**: Extrai portas abertas (`nmap`) ou URLs descobertas (`ffuf`) para arquivos limpos (ex.: `output/nmap_clean_open_ports_example.com.txt`).

### 2. `commands.json`
Especifica comandos CLI para as ferramentas de varredura (`nmap` e `ffuf`). Usado por `cmd/maestro.go` e `data/runner/runner.go`.

- **Formato**:
  ```json
  {
      "nmap": {
          "nmap_slow": "-vvv -sT -sV -sS",
          "nmap_steath": "-sF -Pn --scan-delay 1000ms -f --mtu 24 -D RND:5,ME -p 80,443"
      },
      "ffuf": {
          "default": "ffuf -u FUZZ -w wordlists/wordlist.txt"
      }
  }
  ```
- **Uso**: Define argumentos para varreduras (ex.: `nmap -vvv -sT -sV -sS` ou `ffuf -u FUZZ -w wordlists/wordlist.txt`).

### 3. `db_info.json`
Configura a conexão com o banco de dados PostgreSQL. Usado por `data/db/db_manager.go`.

- **Formato**:
  ```json
  {
      "host": "localhost",
      "port": 5432,
      "user": "postgres",
      "password": "minha_senha",
      "dbname": "bug_hunt_db"
  }
  ```
- **Uso**: Estabelece conexão para inserir resultados limpos em `nmap_open_ports` e `ffuf_endpoints`.

### 4. `env.json`
Define caminhos para resultados e wordlists. Usado por `cmd/maestro.go` e `cmd/show_time.go`.

- **Formato**:
  ```json
  {
      "api_raw_results_path": "output/api_results.json",
      "ai_processed_scopes_path": "output/ai_scopes.json",
      "wordlist_dir": "wordlists/"
  }
  ```
- **Uso**: Especifica onde salvar resultados brutos da API e wordlists para `ffuf`.

### 5. `tokens.json`
Contém credenciais para APIs de plataformas de bug bounty. Usado por `cmd/maestro.go` e `cmd/show_time.go`.

- **Formato**:
  ```json
  {
      "hackerone": {
          "username": "seu_usuario_h1",
          "api_key": "sua_chave_h1"
      },
      "bugcrowd": {
          "token": "seu_token_bugcrowd"
      },
      "intigriti": {
          "token": "seu_token_intigriti"
      },
      "yeswehack": {
          "token": "seu_token_yeswehack"
      }
  }
  ```
- **Uso**: Autentica requisições para coletar alvos de plataformas.

### 6. `selected_platform.json`
Gerado por `cmd/show_time.go` para indicar a plataforma selecionada pelo usuário.

- **Formato**:
  ```json
  {
      "platform": "hackerone"
  }
  ```
- **Uso**: Lido por `cmd/maestro.go` para determinar qual plataforma processar.

## Configuração
1. **Editar arquivos JSON**:
   - Atualize `db_info.json` com as credenciais do seu banco PostgreSQL.
   - Configure `tokens.json` com chaves válidas das plataformas.
   - Verifique `commands.json` para comandos apropriados de `nmap` e `ffuf`.
   - Confirme que `wordlist_dir` em `env.json` aponta para um diretório existente (ex.: `wordlists/` com `wordlist.txt`).
   - `cleaner-templates.json` já contém regex padrão; ajuste se necessário.

2. **Criar wordlist**:
   ```bash
   mkdir wordlists
   echo "http://FUZZ" > wordlists/wordlist.txt
   ```

3. **Configurar o banco**:
   - Use `db_config.sh` para criar o banco `bug_hunt_db` e as tabelas:
     ```sql
     CREATE TABLE scopes (
         platform VARCHAR(50),
         scope TEXT
     );
     CREATE TABLE nmap_open_ports (
         port VARCHAR(10),
         scope_id TEXT
     );
     CREATE TABLE ffuf_endpoints (
         url TEXT,
         scope_id TEXT
     );
     ```

## Notas
- Os arquivos JSON devem estar em `config/json/` para serem carregados corretamente por `utils/json_loader.go`.
- `selected_platform.json` é gerado automaticamente; não edite manualmente.
- Mantenha `wordlist_dir` consistente com o diretório `wordlists/` no projeto.