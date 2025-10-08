# Módulo `runner` - Motor de Execução

**Localização:** `data/runner/runner.go`

## Visão Geral

O `runner` é o "motor" ou os "braços" do sistema AutoHunting. Sua única e crucial responsabilidade é executar ferramentas de segurança externas (como Nmap, Ffuf, etc.) de forma controlada e concorrente. Ele recebe ordens diretas do `Maestro`, executa os comandos contra uma lista de alvos e salva os resultados brutos para processamento posterior pelo módulo `cleaner`.

Ele é projetado para ser robusto, com controle de tempo de execução (timeout) e capacidade de executar múltiplas varreduras simultaneamente para otimizar o tempo total da "caçada".

## Arquitetura e Fluxo de Execução

O `runner` é um módulo especializado que é ativado durante a etapa `RunScanners` do fluxo do `Maestro`.

```txt
INÍCIO DA EXECUÇÃO DO SCANNER

(1) Ativação pelo Orquestrador (`maestro.go`)
    -> O `Maestro` chega na etapa "RunScanners".
    -> Invoca a função `runner.Run(tool, argTemplate, targets, outDir)`.
       - `tool`: O nome da ferramenta (ex: "nmap").
       - `argTemplate`: O comando com placeholders (ex: "-p- -sV {TARGET}").
       - `targets`: A lista de alvos a serem escaneados.
       - `outDir`: O diretório para salvar os resultados brutos.

(2) Runner em Ação: Preparação
    -> A função `Run()` cria o diretório de saída, se não existir.
    -> Configura um "pool de workers":
       - Cria um canal `tasks` para distribuir os alvos.
       - Cria um canal `results` para coletar feedback.
       - Inicia um `sync.WaitGroup` para sincronizar a finalização de todas as tarefas.

(3) Runner em Ação: Distribuição e Execução
    -> A função `Run()` inicia várias goroutines (workers).
    -> Envia cada alvo da lista `targets` para o canal `tasks`.

(4) Lógica do Worker (Execução Concorrente)
    -> Cada worker pega um alvo do canal `tasks`.
    -> Substitui os placeholders no `argTemplate` pelo alvo atual (ex: `{TARGET}` vira "example.com").
    -> Chama `runCommandContext()` para executar o comando da ferramenta com um timeout definido.

(5) Processamento e Armazenamento do Resultado
    -> O worker captura a saída bruta (stdout) do comando.
    -> Gera um nome de arquivo único (ex: "nmap_example.com_timestamp.txt").
    -> Salva a saída bruta nesse arquivo dentro do diretório `outDir`.
    -> Se a ferramenta for `nmap`, ele também tenta fazer um parse do XML para um formato mais legível no log.
    -> Envia uma mensagem de status para o canal `results`.

(6) Finalização
    -> A função `Run()` aguarda (usando `wg.Wait()`) que todos os workers terminem.
    -> Coleta e imprime todas as mensagens do canal `results`.
    -> Retorna o controle para o `Maestro`, que pode então prosseguir para a próxima etapa (ex: "CleanResults").

FIM DA EXECUÇÃO DO SCANNER
```

## Detalhamento das Funções Principais

### `Run(tool, argTemplate string, targets []string, outDir string) error`

É a função principal e ponto de entrada do módulo. Ela orquestra toda a lógica de execução concorrente.

-   **Parâmetros**:
    -   `tool`: O nome do executável da ferramenta (ex: "nmap").
    -   `argTemplate`: A string de argumentos, contendo placeholders como `{TARGET}` ou `{IP}`.
    -   `targets`: Um slice de strings, onde cada string é um alvo para a varredura.
    -   `outDir`: O caminho do diretório onde os arquivos de saída brutos serão salvos.
-   **Lógica**: Configura e gerencia um pool de workers para processar a lista de `targets` em paralelo, garantindo que os resultados sejam salvos corretamente.

### `worker` (Função anônima dentro de `Run`)

Esta é a função executada por cada goroutine. Ela contém a lógica de uma única tarefa de varredura.

-   Recebe um alvo do canal de tarefas.
-   Prepara os argumentos do comando substituindo os placeholders.
-   Chama `runCommandContext` para executar a ferramenta com um timeout.
-   Salva a saída bruta em um arquivo de texto no diretório de resultados.

### `runCommandContext(ctx context.Context, bin string, args ...string) ([]byte, error)`

Uma função utilitária que executa um comando externo. Sua principal vantagem é a integração com `context.Context`, o que permite cancelar a execução do comando se ele demorar mais do que o timeout definido, evitando que o sistema fique preso em uma varredura infinita.

### `parseNmapXML(xmlBytes []byte, target string) string`

Uma função especializada para tratar a saída XML do Nmap (`-oX`). Ela faz o parse do XML e formata as informações de portas, serviços e status de uma maneira mais legível para os logs, facilitando a depuração.

### `sanitizeFilename(s string) string`

Função de segurança e conveniência que remove caracteres inválidos de uma string (como `:`, `/`, `*`) para que ela possa ser usada como um nome de arquivo válido e seguro no sistema de arquivos.

## Estruturas de Dados (`structs`)

-   **`NmapRun`, `Host`, `Port`, etc.**: Um conjunto de estruturas de dados usadas exclusivamente pela função `parseNmapXML`. Elas mapeiam a estrutura do arquivo de saída XML gerado pelo Nmap, permitindo que o Go decodifique o XML de forma nativa.