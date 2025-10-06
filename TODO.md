# AutoHunting - Roteiro de Implementação (TODO)

Este arquivo serve como um mapa para o desenvolvimento futuro do projeto, listando as tarefas pendentes e melhorias planejadas. Itens marcados com `[x]` foram concluídos.

## Configuração e Dados Pendentes

- [x] **Credenciais de API:** Preencher o arquivo `json/tokens.json` com as chaves de API válidas para as plataformas de Bug Bounty (HackerOne, Bugcrowd, etc.).
- [ ] **Conexão com Banco de Dados:** Configurar as informações de conexão (host, usuário, senha) no arquivo `json/db_info.json`.
- [ ] **Variáveis de Ambiente:** Ajustar os caminhos de diretórios e arquivos no `json/env.json` para corresponder à estrutura do ambiente local.
- [ ] **Comandos de Ferramentas:** Revisar e personalizar os argumentos de linha de comando para as ferramentas (Nmap, Ffuf) no arquivo `json/commands.json`.

## Módulo `api` (`api/request_api.go`)

- [ ] **Otimização de Performance:** Paralelizar a busca de escopos do HackerOne.
  - *Detalhes:* Atualmente, a busca é sequencial para cada *handle*. Usar goroutines e um `sync.WaitGroup` para buscar múltiplos handles simultaneamente. Garantir que a escrita no slice de resultados seja *thread-safe* (usando um `sync.Mutex`).

- [ ] **Expansão de Funcionalidade:** Implementar a coleta de escopos real para as outras plataformas (atualmente são placeholders).
  - [ ] Bugcrowd
  - [ ] Intigriti
  - [ ] YesWeHack

## Módulo `maestro` (`cmd/maestro/main.go`)

- [ ] **Otimização de Performance:** Executar scanners em paralelo na etapa `stepRunScanners`.
  - *Detalhes:* Disparar os runners para Nmap, Ffuf, etc., em goroutines separadas para que executem ao mesmo tempo.

- [ ] **Otimização de Performance:** Paralelizar a limpeza de resultados na etapa `stepCleanResults`.
  - *Detalhes:* Criar um pool de workers para processar múltiplos arquivos de resultado simultaneamente.

- [ ] **Otimização de Performance:** Paralelizar a inserção no banco de dados na etapa `stepStoreResults`.
  - *Detalhes:* Usar um pool de workers para ler os arquivos limpos e preparar as transações de inserção no DB em paralelo.

## Módulo `show_time` (`cmd/show_time.go`)

- [x] **Refatoração:** Melhorar a função `checkAPIStatus` para evitar repetição de código.
  - *Detalhes:* Criar uma função auxiliar `testEndpoint(platformName, url)` que centralize a lógica de fazer a requisição GET e verificar o status code.

## Módulo de Banco de Dados (`data/db/db_manager.go`)

- [ ] **Flexibilidade:** Abstrair o dialeto SQL para suportar diferentes bancos de dados.
  - *Detalhes:* A query de inserção usa placeholders do PostgreSQL (`$1`, `$2`). Adaptar a geração da query para usar `?` quando o driver for MySQL/SQLite, baseando-se em `db.DriverName()`.

## Módulo `runner` (`data/runner/runner.go`)

- [ ] **Configurabilidade:** Tornar o número de workers e o timeout configuráveis.
  - *Detalhes:* Atualmente, o número de workers (5) e o timeout (60s) estão fixos no código. Mover esses valores para o `commands.json` ou `env.json` para permitir ajustes sem recompilar.

- [ ] **Melhoria no Tratamento de Erros:** Capturar a saída de erro (`stderr`) dos comandos.
  - *Detalhes:* A função `runCommandContext` redireciona o `stderr` para o console, mas não o captura. Modificá-la para capturar `stderr` e incluir a mensagem de erro no log de resultados, facilitando a depuração de falhas nos scanners.

- [ ] **Flexibilidade de Argumentos:** Refatorar a construção de argumentos para ser mais flexível.
  - *Detalhes:* A substituição de `FUZZ` para o `ffuf` é rígida. Implementar um sistema de placeholders mais genérico (ex: `{TARGET}`, `{WORDLIST}`) que possa ser usado em `commands.json` para diferentes ferramentas.

## Planos de Execução (`json/order-templates.json`)

- [ ] **Criação de Novas Ordens:** Adicionar novos planos de execução para permitir fluxos de trabalho mais flexíveis.
  - [ ] **`apiOnly`**: Um plano que executa apenas a etapa `RequestAPI` para coletar escopos sem iniciar varreduras.
  - [ ] **`scanOnly`**: Um plano que pula a coleta de API e executa as etapas `RunScanners`, `CleanResults` e `StoreResults` em um arquivo de alvos já existente.
  - [ ] **`reconLight`**: Um plano de reconhecimento leve, que usa comandos de scanner mais rápidos (ex: `nmap_fast` em vez de `nmap_slow`).
  - [ ] **`fullHunt`**: Revisar e garantir que o plano de caçada completo (`fullHunt`) inclua todas as etapas na ordem correta.

---
*Este arquivo foi gerado para facilitar o controle de implementação.*