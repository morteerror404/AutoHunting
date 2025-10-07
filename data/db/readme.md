# Módulo `db` - Gerenciador de Banco de Dados

## Descrição

O `db_manager.go` (dentro do pacote `db`) é a camada de persistência de dados do `AutoHunting`. Sua principal responsabilidade é gerenciar a conexão com o banco de dados e inserir os resultados limpos e processados, que foram gerados pelas etapas anteriores do pipeline.

Ele é projetado para ser flexível, carregando as configurações de conexão e os comandos SQL de um arquivo externo (`db_info.json`), o que permite suportar diferentes sistemas de gerenciamento de banco de dados (SGBDs) como PostgreSQL e SQLite.

## Funcionalidades Principais

- **Conexão Flexível**: Conecta-se a diferentes tipos de banco de dados (atualmente PostgreSQL e SQLite) com base na configuração.
- **Configuração Externa**: Carrega todas as informações de conexão (host, usuário, senha) e templates de comandos SQL do arquivo `db_info.json`, desacoplando a lógica do código.
- **Processamento de Resultados**: Lê os arquivos de resultados limpos (ex: `nmap_open_ports_clean.txt`) gerados pelo módulo `cleaner`.
- **Mapeamento Dinâmico de Tabelas**: Determina a tabela de destino no banco de dados de forma inteligente, com base no nome do arquivo de entrada (ex: `nmap_open_ports_clean.txt` -> tabela `nmap_open_ports`).
- **Inserção Transacional**: Insere os dados dentro de uma transação, garantindo que todos os registros de um arquivo sejam inseridos com sucesso ou que nenhuma alteração seja feita em caso de erro (atomicidade).

## Descrição das Funções

- **`ConnectDB()`**
  - **Propósito**: Estabelecer e verificar uma conexão com o banco de dados.
  - **Funcionamento**: Lê o `db_info.json` para obter o tipo de banco de dados e as credenciais. Constrói a string de conexão apropriada para o SGBD (PostgreSQL ou SQLite). Abre a conexão e executa um `Ping()` para garantir que o banco de dados está acessível antes de retornar o objeto de conexão.

- **`ProcessCleanFile(filename, db)`**
  - **Propósito**: Processar um único arquivo de resultado limpo e inserir seus dados no banco de dados.
  - **Funcionamento**:
    1.  Recebe o caminho de um arquivo limpo e uma conexão de banco de dados ativa.
    2.  Analisa o nome do arquivo (ex: `nmap_open_ports_clean.txt`) para extrair o nome da ferramenta (`nmap`) и o nome do template (`open_ports`), que juntos formam o nome da tabela (`nmap_open_ports`).
    3.  Inicia uma transação no banco de dados (`db.Begin()`).
    4.  Lê o arquivo linha por linha. Cada linha é dividida por `|` para obter os campos.
    5.  Constrói dinamicamente uma instrução `INSERT` com base no número de campos encontrados.
    6.  Executa a inserção dentro da transação.
    7.  Se todas as linhas forem processadas sem erro, a transação é confirmada (`tx.Commit()`). Se ocorrer qualquer erro, a transação é revertida (`defer tx.Rollback()`), garantindo a integridade dos dados.

- **`getCommandsConfig(dbType, dbInfo)`**
  - **Propósito**: Função utilitária para obter os templates de comando SQL corretos para um tipo de banco de dados.
  - **Funcionamento**: Recebe o tipo de SGBD (ex: "postgres") e a configuração carregada. Retorna a estrutura `CommandsConfig` correspondente, que contém os comandos SQL para aquele dialeto.

- **`DBInfo`, `DBConfig`, `CommandsConfig` (structs)**
  - **Propósito**: Mapear a estrutura do arquivo `db_info.json` para structs do Go.
  - **Funcionamento**: Permitem que o Go decodifique o JSON de configuração em um formato nativo e de fácil acesso, tornando o código mais limpo e seguro.

## Fluxo de Funcionamento

```txt
INÍCIO DA PERSISTÊNCIA DE DADOS

(1) Ativação pelo Orquestrador (`maestro.go`)
    -> O `maestro` executa o passo `stepStoreResults`.

(2) Conexão com o Banco de Dados
    -> `stepStoreResults` invoca `db.ConnectDB()`.
    -> `ConnectDB` lê `db_info.json` e estabelece a conexão com o SGBD configurado.

(3) Iteração sobre Arquivos Limpos
    -> O `maestro` lê o diretório de resultados limpos (`tool_cleaned_dir`).
    -> Para cada arquivo que corresponde ao padrão `*_clean_*.txt`, ele invoca `db.ProcessCleanFile()`.

(4) Processamento e Inserção
    -> `ProcessCleanFile` analisa o nome do arquivo para determinar a tabela de destino.
    -> Inicia uma transação.
    -> Lê cada linha do arquivo, constrói uma query `INSERT` e a executa.
    -> Ao final do arquivo, commita a transação.

(5) Conclusão
    -> Após processar todos os arquivos, o `maestro` fecha a conexão com o banco de dados.

FIM DA PERSISTÊNCIA DE DADOS
```