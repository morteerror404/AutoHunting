# Módulo `runner` - Executor de Ferramentas de Varredura

## Descrição

O `runner.go` (dentro do pacote `runner`) é o motor de execução do `AutoHunting`. Sua responsabilidade é pegar a lista de alvos, já processada e unificada pelo módulo `results`, e executar ferramentas de varredura externas (como Nmap e Ffuf) contra cada um desses alvos.

Ele opera de forma concorrente, utilizando um pool de workers (goroutines) para realizar múltiplas varreduras em paralelo, otimizando drasticamente o tempo total do processo de reconhecimento. Os resultados brutos de cada ferramenta são salvos em arquivos individuais no diretório "dirt" para serem processados pela próxima etapa do pipeline, o `cleaner`.

## Funcionalidades Principais

- **Execução Paralela**: Utiliza um pool de workers para escanear múltiplos alvos simultaneamente, aumentando a eficiência.
- **Suporte a Múltiplas Ferramentas**: Estruturado para executar diferentes ferramentas de linha de comando (atualmente `nmap` e `ffuf`).
- **Construção Dinâmica de Comandos**: Monta os argumentos de comando específicos para cada ferramenta e alvo, como a definição do arquivo de saída XML (`-oX`) para o Nmap.
- **Controle de Timeout**: Cada execução de comando tem um timeout definido, evitando que o processo inteiro fique travado por uma única varredura lenta ou com falha.
- **Armazenamento de Resultados Brutos**: Salva a saída completa (stdout) de cada ferramenta em um arquivo único, nomeado de forma a identificar a ferramenta e o alvo, garantindo que nenhum dado seja perdido.
- **Parsing Preliminar (Nmap)**: Inclui lógica para fazer o parsing do XML de saída do Nmap, fornecendo um relatório imediato e legível no console.

## Descrição das Funções

- **`Run(targetsFile, args, outDir, tool)`**
  - **Propósito**: É o ponto de entrada principal do módulo, orquestrando a execução de uma ferramenta específica.
  - **Funcionamento**: Lê o arquivo de alvos, cria o diretório de saída e configura um pool de workers. Distribui os alvos para os workers através de um canal (`tasks`) e aguarda a conclusão de todos os trabalhos.

- **`worker(id, tasks, results, ...)`**
  - **Propósito**: A função executada por cada goroutine. É o verdadeiro executor da varredura.
  - **Funcionamento**: Fica em um loop, recebendo alvos do canal `tasks`. Para cada alvo, constrói os argumentos do comando, executa a ferramenta usando `runCommandContext`, processa a saída (fazendo o parsing do XML do Nmap, se aplicável) e salva o resultado bruto em um arquivo.

- **`runCommandContext(ctx, bin, args...)`**
  - **Propósito**: Função utilitária para executar um comando externo com um `context` (para controle de timeout).
  - **Funcionamento**: Encapsula a chamada `exec.CommandContext`, que permite que o comando seja finalizado se o `context` for cancelado (por exemplo, por um timeout).

- **`parseNmapXML(xmlBytes, target)`**
  - **Propósito**: Converter a saída XML complexa do Nmap em um relatório de texto simples e legível.
  - **Funcionamento**: Usa as structs `NmapRun`, `Host`, `Port`, etc., para decodificar o XML em estruturas de dados do Go. Em seguida, itera sobre essas estruturas para formatar um relatório amigável, mostrando o status do host e as portas abertas com seus serviços.

- **`sanitizeFilename(s)`**
  - **Propósito**: Garantir que um alvo (como uma URL ou IP) possa ser usado como parte de um nome de arquivo válido.
  - **Funcionamento**: Substitui caracteres inválidos em nomes de arquivo (como `:`, `/`, `*`) por underscores.

- **`NmapRun`, `Host`, `Port`, etc. (structs)**
  - **Propósito**: Mapear a estrutura do arquivo de saída XML do Nmap.
  - **Funcionamento**: As tags `xml:"..."` em cada campo da struct dizem ao decodificador `xml.Unmarshal` como associar os elementos e atributos do XML aos campos da struct, automatizando o parsing.

## Fluxo de Funcionamento

```txt
INÍCIO DA VARREDURA

(1) Ativação pelo Orquestrador (`maestro.go`)
    -> O `maestro` executa o passo `stepRunScanners`.
    -> Para cada ferramenta a ser executada (ex: nmap, ffuf), ele invoca `runner.Run()`.
    -> Parâmetros fornecidos:
        - `targetsFile`: O caminho para o arquivo de alvos unificado (ex: `targets_for_scanning.txt`).
        - `args`: Os argumentos de linha de comando para a ferramenta (ex: "-p- -sV").
        - `outDir`: O diretório onde os resultados brutos serão salvos (`tool_dirt_dir`).
        - `tool`: O nome da ferramenta a ser executada ("nmap").

(2) Configuração do Runner
    -> `runner.Run` lê os alvos e inicia um pool de workers (ex: 5 goroutines).
    -> Os alvos são enviados para um canal de tarefas.

(3) Execução Concorrente
    -> Cada worker pega um alvo do canal.
    -> Monta o comando completo (ex: `nmap -p- -sV -oX <output_path> <target_ip>`).
    -> Executa o comando com um timeout.
    -> Salva a saída XML bruta no `outDir`.
    -> Imprime um resumo formatado no console.

(4) Próxima Etapa no Fluxo
    -> Os arquivos de resultado brutos (XML, JSON, etc.) gerados no `tool_dirt_dir` se tornam a entrada para o módulo `cleaner`, que irá extrair as informações úteis.

FIM DA VARREDURA
```