# Módulo `results` - Unificador e Validador de Escopo

## Descrição

O `process_results.go` (dentro do pacote `results`) atua como o módulo centralizador e validador dos dados de escopo coletados. Sua principal responsabilidade é unificar os alvos provenientes de diferentes fontes, como as APIs de plataformas de Bug Bounty (`Request_API.go`) e, futuramente, os resultados interpretados por IA (`AI_scope_interpreter.go`).

Este componente garante que a lista de alvos que alimenta os scanners (`runner.go`) seja limpa, desduplicada e formatada corretamente, otimizando a eficiência e a precisão do fluxo de trabalho de reconhecimento.

## Funcionalidades Principais

- **Unificação de Fontes**: Consolida listas de alvos (domínios, IPs, URLs) de múltiplas fontes em um único conjunto de dados.
- **Validação e Limpeza**: Remove entradas duplicadas, inválidas ou fora do escopo, garantindo a integridade dos dados.
- **Formatação**: Prepara os dados no formato exato esperado pela próxima etapa do pipeline, como um arquivo de texto com um alvo por linha.
- **Ponte Estratégica**: Atua como uma ponte crucial entre a coleta de escopo (o "quê" varrer) e a execução das ferramentas (o "como" varrer).

## Fluxo de Funcionamento

```txt
INÍCIO DO PROCESSAMENTO DE ESCOPO

(1) Fontes de Dados de Entrada
    - `api/request_api.go`: Gera um arquivo com uma lista de alvos brutos (ex: `api_results.txt`).
    - `ai/interpreter.go` (Futuro): Gera um arquivo com alvos extraídos de políticas de texto.

(2) Ativação pelo Orquestrador
    - `cmd/maestro.go`: Após a etapa de coleta (`RequestAPI`), invoca uma função do módulo `results`.
      -> Exemplo de chamada: `results.ProcessScopes(inputPaths, outputPath)`

(3) Módulo `results` em Ação
    -> A função `ProcessScopes` lê os arquivos de entrada (ex: `api_results.txt`).
    -> Consolida todos os alvos em uma única estrutura de dados em memória (ex: um mapa para desduplicação automática).
    -> Itera sobre os alvos consolidados, aplicando regras de validação:
        - Remove linhas em branco.
        - Verifica se o formato do alvo é válido (ex: é um domínio ou IP válido).
        - Remove duplicatas.

(4) Geração da Saída Limpa
    -> O módulo `results` escreve a lista final de alvos, já limpa e validada, em um arquivo de saída.
    -> Este arquivo (ex: `targets_for_scanning.txt`) servirá como entrada para o módulo `runner.go`.

(5) Próxima Etapa no Fluxo
    -> O `maestro.go` prossegue para a etapa `RunScanners`, passando o caminho do arquivo de alvos limpo para o `runner.go`.

FIM DO PROCESSAMENTO DE ESCOPO
```