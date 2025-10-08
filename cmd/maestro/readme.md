# Maestro

**Localização:** `cmd/maestro/main.go`

## Visão Geral

O `Maestro` é o cérebro e o executor central do sistema AutoHunting. Ele é projetado para operar sem interação direta do usuário, funcionando como um serviço de backend que recebe ordens, executa tarefas complexas e registra seu progresso de forma detalhada.

Sua principal responsabilidade é orquestrar o fluxo de trabalho de uma "caçada" (hunt), desde a coleta de escopos até o armazenamento dos resultados, seguindo um plano de execução pré-definido.

## Arquitetura e Fluxo de Execução

O `Maestro` opera com base em uma arquitetura de comando e controle, onde o `show_time` atua como o controlador e o `Maestro` como o executor.

```txt
INÍCIO DA ORQUESTRAÇÃO

(1) Ativação pelo Controlador (`show_time`)
    -> O `show_time` coleta a entrada do usuário (ex: plataforma e tarefa).
    -> Invoca `utils.CreateExecutionOrder()` para gerar o arquivo `order.json` a partir de um template.
    -> Executa o binário do `Maestro` e começa a monitorar o `maestro_execution.log`.

(2) Maestro em Ação: Inicialização
    -> O `Maestro` é iniciado.
    -> Chama `setupLogging()` para configurar os arquivos de log (`maestro_execution.log` e `maestro_summary.json`).
    -> Chama `loadAllConfigs()` para carregar todos os arquivos JSON de configuração (`env.json`, `tokens.json`, `commands.json`) e o `order.json`.

(3) Maestro em Ação: Execução dos Passos
    -> O `Maestro` entra em um loop, iterando sobre cada "step" definido no `order.json`.
    -> Para cada passo (ex: "RequestAPI"):
        - Loga "Iniciando passo: Coletar escopos da API...".
        - Invoca a função correspondente (ex: `stepRequestAPI()`).

(4) Delegação para Módulos Especializados
    -> A função de passo (ex: `stepRequestAPI()`) delega a tarefa para o módulo apropriado (ex: `api.RunRequestAPI(...)`).
    -> O módulo especializado (ex: `api`) executa sua lógica, utilizando as configurações passadas pelo `Maestro`.
    -> O módulo retorna um erro ou `nil` para o `Maestro`.

(5) Registro e Continuação
    -> O `Maestro` recebe o resultado do passo.
    -> Se houver erro, chama `logAndExit()` para registrar a falha e interromper a execução.
    -> Se for sucesso, loga a conclusão do passo e continua para o próximo item na ordem.

(6) Finalização
    -> Após completar todos os passos (ou falhar), a função `saveExecutionLog()` é chamada (via `defer`).
    -> Ela salva o resumo completo da execução no arquivo `maestro_summary.json`.
    -> O processo do `Maestro` encerra, sinalizando para o `show_time` que a tarefa terminou.

FIM DA ORQUESTRAÇÃO
```

## Detalhamento das Funções Principais

A seguir, um detalhamento das funções mais importantes encontradas em `maestro.go`.

### `main()`

Ponto de entrada do programa. Sua única função é chamar `runMaestro()` e sair com um código de status apropriado (0 para sucesso, 1 para erro).

### `runMaestro() error`

É o coração do `Maestro`. Orquestra toda a execução.

-   **Inicializa o Contexto**: Cria a struct `MaestroContext`, que armazenará todo o estado da execução (configurações, logs, ordem, etc.).
-   **Configura o Logging**: Chama `setupLogging()` para preparar os arquivos de log.
-   **Carrega Configurações**: Chama `loadAllConfigs()` para preencher o contexto com as informações de todos os arquivos JSON.
-   **Processa os Passos**: Itera sobre os `steps` da ordem de execução e, usando um `switch`, chama a função de passo (`step...`) correspondente.
-   **Gerencia o Log de Execução**: Após cada passo bem-sucedido, adiciona um registro de `Success` ao `ctx.Logs`. Em caso de erro, chama `logAndExit()` para registrar a falha e interromper o fluxo.
-   **Finaliza**: Registra o tempo total de processamento e retorna `nil` em caso de sucesso.

### Funções de Passo (`step...`)

Cada uma dessas funções recebe o `*MaestroContext` e é responsável por uma etapa específica do fluxo de trabalho.

-   **`stepRequestAPI(ctx *MaestroContext) error`**:
    -   Delega a tarefa para `api.RunRequestAPI`.
    -   Usa as configurações de `tokens.json` e `env.json` para buscar os escopos da plataforma especificada na ordem e salvá-los em um arquivo de resultado bruto.

-   **`stepRunScanners(ctx *MaestroContext) error`**:
    -   Delega a tarefa para `runner.Run`.
    -   Lê os escopos do arquivo gerado pelo `stepRequestAPI`.
    -   Executa as ferramentas de scanning (ex: Nmap, Ffuf) com os argumentos definidos em `commands.json`.
    -   Salva a saída bruta de cada ferramenta em seu respectivo arquivo no diretório de resultados brutos (`tool_dirt_dir`).

-   **`stepCleanResults(ctx *MaestroContext) error`**:
    -   Delega a tarefa para `cleaner.CleanFile`.
    -   Varre o diretório de resultados brutos.
    -   Para cada arquivo de resultado, aplica um template de limpeza para extrair informações úteis (ex: portas abertas, endpoints encontrados).
    -   Salva os dados limpos em um novo arquivo no diretório de resultados limpos (`tool_cleaned_dir`).

-   **`stepStoreResults(ctx *MaestroContext) error`**:
    -   Delega a tarefa para `db.ProcessCleanFile`.
    -   Conecta-se ao banco de dados usando as credenciais de `db_info.json`.
    -   Varre o diretório de resultados limpos.
    -   Para cada arquivo, lê os dados e os insere na tabela apropriada no banco de dados.

-   **`stepInsertScope(ctx *MaestroContext) error`**:
    -   Executa uma tarefa administrativa de inserir um único escopo no banco de dados.
    -   Lê a plataforma e o escopo do campo `Data` da ordem de execução.
    -   Conecta-se ao DB e executa a query `INSERT`.

-   **`stepListScopes(ctx *MaestroContext) error`**:
    -   Executa uma tarefa administrativa de listar escopos.
    -   Lê a plataforma da ordem de execução.
    -   Conecta-se ao DB, consulta os escopos e os imprime no log de execução (que é exibido para o usuário pelo `show_time`).

### Funções de Suporte

-   **`setupLogging(ctx *MaestroContext) error`**:
    -   Lê `env.json` para encontrar o `log_dir`.
    -   Cria o diretório de log, se não existir.
    -   Abre o arquivo `maestro_execution.log` no modo de escrita, truncando o conteúdo anterior.
    -   Configura o `log` padrão do Go para escrever simultaneamente no console (`os.Stdout`) e no arquivo de log.

-   **`loadAllConfigs(ctx *MaestroContext) error`**:
    -   Função utilitária que carrega sequencialmente `env.json`, `commands.json`, `tokens.json` e o arquivo `order.json`, populando a struct `MaestroContext`.

-   **`logAndExit(ctx *MaestroContext, step string, err error)`**:
    -   Registra uma mensagem de erro detalhada no log.
    -   Adiciona uma entrada de `Failed` ao log de resumo (`ctx.Logs`), incluindo a mensagem de erro.

-   **`saveExecutionLog(ctx *MaestroContext)`**:
    -   Geralmente chamada via `defer` em `runMaestro`.
    -   Serializa a slice `ctx.Logs` para o formato JSON.
    -   Salva o resultado no arquivo `maestro_summary.json` dentro do diretório de logs.

## Estruturas de Dados (`structs`)

-   **`MaestroContext`**: O contêiner principal que carrega o estado e as configurações durante toda a execução do `Maestro`.
-   **`Config`**: Mapeia a estrutura do arquivo `env.json`.
-   **`Commands`**: Mapeia a estrutura do arquivo `commands.json`.
-   **`Tokens`**: Mapeia a estrutura do arquivo `tokens.json`.
-   **`ExecutionLog`**: Define a estrutura de uma entrada no log de resumo (`maestro_summary.json`).
-   **`MaestroOrder`**: Mapeia a estrutura do arquivo de ordem (`order.json`) que dita o comportamento do `Maestro`.