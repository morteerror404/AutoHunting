# AutoHunting - Roteiro de Implementação (TODO)

Este arquivo serve como um mapa para o desenvolvimento futuro do projeto, listando as tarefas pendentes e melhorias planejadas. Marque os itens com `[x]` conforme forem concluídos.

## Módulo `api` (`api/request_api.go`)

- [ ] **Otimização de Performance:** Paralelizar a busca de escopos do HackerOne.
  - *Detalhes:* Atualmente, a busca é sequencial para cada *handle*. Usar goroutines e um `sync.WaitGroup` para buscar múltiplos handles simultaneamente. Garantir que a escrita no slice de resultados seja *thread-safe* (usando um `sync.Mutex`).

- [ ] **Expansão de Funcionalidade:** Implementar a coleta de escopos para as outras plataformas.
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

- [ ] **Refatoração:** Melhorar a função `checkAPIStatus` para evitar repetição de código.
  - *Detalhes:* Criar uma função auxiliar `testEndpoint(platformName, url)` que centralize a lógica de fazer a requisição GET e verificar o status code.

## Módulo de Banco de Dados (`data/db/db_manager.go`)

- [ ] **Flexibilidade:** Abstrair o dialeto SQL para suportar diferentes bancos de dados.
  - *Detalhes:* A query de inserção usa placeholders do PostgreSQL (`$1`, `$2`). Adaptar a geração da query para usar `?` quando o driver for MySQL/SQLite, baseando-se em `db.DriverName()`.

---
*Este arquivo foi gerado para facilitar o controle de implementação.*