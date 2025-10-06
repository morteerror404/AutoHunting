### Como funciona o process_results.go ? 

pipipi popopo

### Data-Flow da ferramenta 
```txt

NOVO FLUXO DE EXECUÇÃO COM VERIFICAÇÃO DE API

1. Ponto de Partida
cmd/maestro.go / cmd/show_time.go
  -> CHAMA -> data_manager.InitializePaths() (dentro de process_results.go)
  -> data_manager.InitializePaths() CHAMA -> utils/json_loader.go
  -> utils/json_loader.go LÊ -> json/env.json (paths de pastas/APIs)
  -> utils/json_loader.go LÊ -> json/db_info.json

2. Seleção de Plataforma
cmd/show_time.go
  -> CHAMA -> data_manager.DisplayPlatformMenu() (dentro de process_results.go)
  
data_manager.DisplayPlatformMenu()
  -> CHAMA -> data_manager.checkAPIStatus() (Verifica a conexão/ping)
  -> data_manager.checkAPIStatus() APRESENTA o menu ao usuário com o status do ping.
  -> **RETORNA** a plataforma escolhida para cmd/show_time.go.

3. Processamento de Dados da API (Exemplo)
api/request_api.go
  -> Faz a requisição para a URL da plataforma escolhida.
  -> Salva o resultado (XML, JSON, etc.) em: {apiDirtyPath}/resultado.json

4. Portabilidade e Limpeza (Tradução de Extensões)
cmd/maestro.go
  -> CHAMA -> data_manager.TranslateFileExtensions() (dentro de process_results.go)
  -> data_manager.TranslateFileExtensions()
      -> LÊ arquivos de: {apiDirtyPath}
      -> EXECUTA a tradução de extensão (Ex: .json -> .txt)
      -> MOVE arquivos traduzidos para: {apiCleanPath}
```