# Show Time

**Localização:** `cmd/show_time/main.go`

## Visão Geral

`Show Time` é a interface de linha de comando (CLI) e o ponto de entrada para o usuário do sistema AutoHunting. Ele atua como o "controlador" ou "rosto" do projeto, responsável por apresentar menus, coletar as escolhas do usuário e traduzir essas escolhas em ordens claras para o `Maestro`.

Sua principal responsabilidade é gerenciar a interação com o usuário, delegando todas as tarefas de execução pesada (como chamadas de API, scanning e operações de banco de dados) para o `Maestro`.

## Arquitetura e Fluxo de Execução

`Show Time` implementa a parte do usuário no padrão de arquitetura de comando e controle do sistema.

```txt
INÍCIO DA INTERAÇÃO COM O USUÁRIO

(1) Apresentação do Menu
    -> `showMainMenu()` é chamado, exibindo as opções principais para o usuário (Iniciar Caçada, Consultar DB, etc.).

(2) Coleta da Escolha do Usuário
    -> O programa aguarda a entrada do usuário.
    -> A escolha é lida e a função de tratamento correspondente é chamada (ex: `handleHuntMenu()`).

(3) Coleta de Parâmetros
    -> A função de tratamento (ex: `handleHuntMenu()`) pode solicitar informações adicionais, como a plataforma, chamando `selectPlatform()`.

(4) Criação da Ordem de Execução
    -> Com todas as informações necessárias, `show_time` invoca `utils.CreateExecutionOrder()`.
    -> Esta função seleciona um template de `order-templates.json` e o preenche com os dados do usuário (plataforma, escopo, etc.), gerando o arquivo `order.json`.

(5) Ativação do Maestro
    -> A função `triggerMaestro()` é chamada.
    -> Ela executa o binário do `Maestro` em um novo processo.

(6) Monitoramento em Tempo Real
    -> `triggerMaestro()` inicia uma goroutine que executa `tailLogFile()`.
    -> `tailLogFile()` "assiste" ao arquivo `maestro_execution.log` e imprime cada nova linha no console do usuário, prefixada com "[Maestro]".
    -> Isso fornece um feedback em tempo real sobre o progresso da tarefa.

(7) Finalização
    -> O processo principal de `show_time` aguarda o término do processo do `Maestro`.
    -> Quando o `Maestro` termina, `triggerMaestro()` encerra a goroutine de monitoramento e exibe uma mensagem final de sucesso ou falha.
    -> O ciclo recomeça, exibindo o menu principal novamente.

FIM DA INTERAÇÃO
```

## Detalhamento das Funções Principais

A seguir, um detalhamento das funções mais importantes encontradas em `show_time.go`.

### `main()`

Ponto de entrada do programa. Inicia um loop infinito que chama `showMainMenu()`, garantindo que o usuário sempre retorne ao menu principal após a conclusão de uma tarefa.

### `showMainMenu()`

Imprime o menu principal com as opções de alto nível. Lê a entrada do usuário e usa um `switch` para direcionar o fluxo para a função de tratamento apropriada (ex: `handleHuntMenu`, `handleDBMenu`).

### Funções de Tratamento (`handle...`)

Estas funções gerenciam os submenus e a lógica para cada opção principal.

-   **`handleHuntMenu()`**: Orquestra o início de uma "caçada completa". Chama `selectPlatform` para obter o alvo, cria uma ordem de execução com a tarefa `fullHunt` e, por fim, chama `triggerMaestro`.

-   **`handleDBMenu()`**: Apresenta as opções relacionadas ao banco de dados (listar escopos, inserir escopo). Para cada opção, ele coleta os dados necessários, cria a ordem de execução correspondente (`listScopes` ou `insertScope`) e chama `triggerMaestro`.

-   **`handleAPIStatusMenu()`**: Permite ao usuário verificar rapidamente a conectividade com as APIs das plataformas. Chama `selectPlatform` e depois `checkAPIStatus`.

-   **`handleManualScopeInsertion()`**: Guia o usuário no processo de adicionar um novo escopo manualmente. Coleta a plataforma e o escopo, cria uma ordem de execução `insertScope` com esses dados e dispara o `Maestro`.

### Funções de Suporte

-   **`loadTokens() (Tokens, error)`**: Carrega e decodifica o arquivo `tokens.json` para uma struct `Tokens`, disponibilizando as chaves de API para outras funções.

-   **`selectPlatform(tokens Tokens) (string, error)`**: Uma função utilitária crucial. Ela verifica quais plataformas têm tokens configurados no arquivo `tokens.json`, exibe um menu numerado para o usuário e retorna a string da plataforma selecionada.

-   **`checkAPIStatus(platform string) error`**: Realiza uma requisição `GET` simples para um endpoint público conhecido da plataforma selecionada para verificar se a API está online e respondendo com um status `200 OK`.

### Orquestração do Maestro

-   **`triggerMaestro()`**: Esta é a função central que conecta `show_time` ao `Maestro`.
    -   Executa o binário compilado `./bin/maestro` usando `os/exec`.
    -   Inicia uma goroutine para monitorar o arquivo de log do `Maestro` em tempo real.
    -   Usa um `context.Context` para garantir que a goroutine de monitoramento seja encerrada de forma limpa quando o processo do `Maestro` terminar.
    -   Usa `cmd.Wait()` para bloquear a execução até que o `Maestro` finalize.
    -   Reporta o status final da execução do `Maestro`.

-   **`tailLogFile(ctx context.Context, filepath string)`**: A função executada na goroutine de monitoramento.
    -   Utiliza a biblioteca `github.com/hpcloud/tail` para seguir o arquivo de log de forma eficiente (semelhante ao comando `tail -f`).
    -   Imprime cada nova linha do log no console, permitindo que o usuário acompanhe o progresso.
    -   Para de monitorar quando o `context` é cancelado por `triggerMaestro`.

## Estruturas de Dados (`structs`)

-   **`Tokens`**: Mapeia a estrutura do arquivo `tokens.json`, contendo as credenciais de API para as diferentes plataformas de Bug Bounty.