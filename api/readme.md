# Módulo `api` - Coletor de Escopos de Bug Bounty

## Descrição

O `request_api.go` (dentro do pacote `api`) é o componente responsável por se conectar às APIs de diversas plataformas de Bug Bounty para coletar e baixar os escopos de programas. Ele atua como o ponto de entrada do fluxo de dados do `AutoHunting`, buscando os alvos brutos (domínios, URLs, CIDRs) que serão posteriormente processados e utilizados nas etapas de varredura.

Atualmente, a implementação suporta a plataforma **HackerOne**, com uma estrutura preparada para a expansão futura para outras plataformas como Bugcrowd, Intigriti e YesWeHack.

## Funcionalidades Principais

- **Conexão com APIs**: Autentica e se comunica com os endpoints das plataformas de Bug Bounty utilizando as credenciais fornecidas em `tokens.json`.
- **Extração de Escopo**: Executa as chamadas necessárias para obter a lista de programas e, para cada um, seus ativos (escopos) estruturados.
- **Filtragem Inteligente**: Aplica filtros para extrair apenas os ativos que são elegíveis para submissão e recompensa (`eligible_for_submission` e `eligible_for_bounty`), focando nos tipos de ativos mais relevantes para a automação (`URL`, `DOMAIN`, `CIDR`).
- **Geração de Saída Bruta**: Consolida todos os escopos coletados em um único arquivo de texto (`.txt`), que servirá como fonte de dados "suja" (dirt) para a próxima etapa do pipeline.

## Fluxo de Funcionamento

```txt
INÍCIO DA COLETA DE ESCOPO

(1) Ativação pelo Orquestrador (`maestro.go`)
    -> O `maestro` invoca a função `api.RunRequestAPI`.
    -> Parâmetros fornecidos:
        - `apiDirtResultsPath`: O caminho do arquivo onde os resultados brutos serão salvos.
        - `platform`: A plataforma a ser consultada (ex: "hackerone").
        - `tokens`: As credenciais de API carregadas do `tokens.json`.

(2) Seleção da Plataforma
    -> Dentro de `RunRequestAPI`, um `switch` direciona a execução para a lógica específica da plataforma solicitada.

(3) Coleta de Dados (Exemplo: HackerOne)
    -> A função `fetchHackerOneProgramHandles` é chamada para obter uma lista de todos os programas públicos acessíveis.
    -> O sistema itera sobre cada "handle" (identificador de programa) retornado.
    -> Para cada handle, a função `fetchHackerOneStructuredScopes` é chamada.
    -> A resposta da API é filtrada para manter apenas os ativos que atendem aos critérios de elegibilidade e tipo.

(4) Consolidação e Geração do Arquivo de Saída
    -> Todos os escopos válidos de todos os programas são agregados em uma única lista em memória.
    -> A função `writeLinesToFile` é usada para escrever esta lista no arquivo de saída especificado pelo `maestro` (ex: `results/dirt/api_hackerone_scopes.txt`). Cada escopo é escrito em uma nova linha.

(5) Próxima Etapa no Fluxo
    -> O arquivo de texto gerado se torna a entrada para o módulo `results` (`process_results.go`), que irá normalizar, limpar e unificar estes dados.

FIM DA COLETA DE ESCOPO
