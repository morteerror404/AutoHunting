# Módulo `cleaner` - Inteligência Léxica

## Descrição

O `cleaner.go` é o componente de inteligência léxica do AutoHunting. Sua principal responsabilidade é transformar os resultados brutos e não estruturados, gerados por ferramentas de varredura como Nmap e Nuclei, em dados limpos, formatados e prontos para serem inseridos no banco de dados.

Ele atua como uma ponte entre a fase de execução de ferramentas (`runner`) e a fase de persistência de dados (`db_manager`), garantindo que apenas informações relevantes e bem formatadas sejam armazenadas.

## Funcionalidades Principais

- **Limpeza Baseada em Templates Modulares**: Utiliza um sistema de templates em dois níveis para máxima flexibilidade. O arquivo principal (`cleaner-templates.json`) atua como um índice, mapeando cada ferramenta ao seu próprio arquivo de template (ex: `nmap.json`). Isso torna o módulo altamente extensível, permitindo adicionar suporte a novas ferramentas apenas modificando os arquivos JSON, sem alterar o código Go.
- **Extração com Expressões Regulares (Regex)**: Aplica padrões de regex para identificar e extrair com precisão os dados importantes (como portas abertas, serviços, URLs vulneráveis) do texto bruto.
- **Formato de Saída Estruturado**: Gera arquivos de saída (`.txt`) onde cada linha representa um achado e os campos são separados por um pipe (`|`). Ex: `80|http`.
- **Nomenclatura e Caminhos Inteligentes**: Cria arquivos de saída com nomes descritivos (ex: `nmap_example.com_2023_clean_open_ports.txt`) e os salva no diretório correto (`tool_cleaned_dir`), definido no `env.json`, garantindo a integração com o restante do fluxo de trabalho.

## Estruturas de Dados Principais

O funcionamento do `cleaner` é baseado em um conjunto de `structs` que espelham a estrutura dos arquivos de configuração JSON.

- **`Template`**: Representa uma única regra de limpeza.
  - `Regex`: A expressão regular usada para encontrar e extrair dados de uma linha.
  - `Fields`: Uma lista de nomes para os campos que serão extraídos pelos grupos de captura do regex.

- **`TemplatePaths`**: Mapeia a estrutura do arquivo `cleaner-templates.json`.
  - `Templates`: Um mapa onde a chave é o nome da ferramenta (ex: `"nmap"`) e o valor é o caminho para o arquivo de template específico dessa ferramenta (ex: `"/etc/AutoHunting/templates/cleaner/nmap.json"`).

- **`ToolTemplates`**: Mapeia a estrutura de um arquivo de template de ferramenta individual (ex: `nmap.json`).
  - `Templates`: Um mapa onde a chave é o nome do tipo de limpeza (ex: `"open_ports"`) e o valor é a `struct` `Template` correspondente.

- **`EnvConfig`**: Carrega as configurações de caminho do `env.json`.
  - `Path.ToolCleanedDir`: Armazena o caminho do diretório onde os arquivos limpos devem ser salvos.

## Fluxo de Funcionamento

```txt
INÍCIO DO PROCESSO DE LIMPEZA

(1) Ponto de Entrada: O Orquestrador (maestro.go)
   -> Após a conclusão da etapa "RunScanners", o `maestro.go` identifica que os resultados brutos precisam ser processados.
   -> O `maestro.go` invoca a função `cleaner.CleanFile(filename, templateName)`.
      - `filename`: O caminho completo para o arquivo de resultado bruto (ex: "/etc/AutoHunting/Dirt/nmap_example.com_12345.txt").
      - `templateName`: O nome da regra de limpeza a ser aplicada (ex: "open_ports").

(2) Módulo `cleaner.go`: Carregamento de Configurações Essenciais
   -> O `cleaner.go` é ativado.
   -> **PRIMEIRA AÇÃO:** Ele precisa entender seu ambiente.
   -> CHAMA -> `utils.LoadJSON("env.json", ...)` para carregar as configurações de ambiente.
      - Extrai o valor de `tool_cleaned_dir` (ex: "/etc/AutoHunting/Clean/"), que define onde os arquivos limpos serão salvos.
   -> CHAMA -> `utils.LoadJSON("cleaner-templates.json", ...)` para carregar o "índice" de templates.
      - Este arquivo mapeia nomes de ferramentas para os caminhos de seus arquivos de regras específicos (ex: "nmap" -> "/etc/AutoHunting/templates/cleaner/nmap.json").

(3) Módulo `cleaner.go`: Descoberta do Template Correto
   -> O `cleaner.go` analisa o `filename` recebido.
   -> Ele extrai o nome base do arquivo (ex: "nmap_example.com_12345.txt").
   -> Itera sobre as chaves do índice de templates carregado (nmap, ffuf, httpx, etc.).
   -> Encontra uma correspondência, pois o nome do arquivo começa com "nmap_".
   -> **RESULTADO:** Identifica que a ferramenta é "nmap" e o caminho para suas regras é "/etc/AutoHunting/templates/cleaner/nmap.json".

(4) Módulo `cleaner.go`: Carregamento da Regra de Limpeza Específica
   -> O `cleaner.go` lê o conteúdo do arquivo de template específico da ferramenta (ex: "/etc/AutoHunting/templates/cleaner/nmap.json").
   -> Ele decodifica o JSON deste arquivo.
   -> Usando o `templateName` ("open_ports") recebido na chamada da função, ele localiza a regra exata dentro do JSON.
   - **RESULTADO:** Carrega a expressão regular (`regex`) e os nomes dos campos (`fields`) para a regra "open_ports".
      - `regex`: "^(\\d+)/tcp\\s+open\\s+([^\\s]+)"
      - `fields`: ["port", "service"]

(5) Módulo `cleaner.go`: Processamento e Extração de Dados
   -> O `cleaner.go` abre o arquivo de resultado bruto (ex: "/etc/AutoHunting/Dirt/nmap_example.com_12345.txt").
   -> Ele lê o arquivo linha por linha.
   -> Para cada linha (ex: "80/tcp    open  http"), ele aplica a expressão regular carregada.
   -> Se a linha corresponde, ele extrai os "grupos de captura" (as partes entre parênteses no regex).
      - Grupo 1: "80"
      - Grupo 2: "http"

(6) Módulo `cleaner.go`: Formatação e Estruturação
   -> Os dados extraídos ("80", "http") são unidos em uma única string, usando um pipe "|" como delimitador.
   -> **RESULTADO:** A string formatada "80|http" é criada.
   -> Esta string é adicionada a uma lista (slice) em memória que armazena todas as linhas limpas.

(7) Módulo `cleaner.go`: Persistência do Resultado Limpo
   -> Após processar todas as linhas do arquivo de entrada, o `cleaner.go` prepara-se para salvar os resultados.
   -> Constrói o nome do novo arquivo de saída (ex: "nmap_example.com_12345_clean_open_ports.txt").
   -> Constrói o caminho completo para o arquivo de saída, combinando o diretório de destino com o novo nome (ex: "/etc/AutoHunting/Clean/nmap_example.com_12345_clean_open_ports.txt").
   -> Cria o novo arquivo neste caminho.
   -> Escreve todas as linhas formatadas da lista em memória para o novo arquivo.

(8) Próxima Etapa no Fluxo: Módulo `db_manager.go`
   -> O arquivo de saída limpo e estruturado (ex: "/etc/AutoHunting/Clean/nmap_example.com_12345_clean_open_ports.txt") está agora pronto.
   -> Em uma etapa subsequente, o `maestro.go` invocará o `db_manager.go`, que lerá este arquivo limpo e inserirá os dados no banco de dados.

FIM DO PROCESSO DE LIMPEZA
```

O processo de limpeza é orquestrado pela função `CleanFile(filename, templateName) error`, que segue os seguintes passos detalhados:

1.  **Carregamento de Configurações Iniciais**:
    - O `env.json` é lido para obter o caminho do diretório de saída (`tool_cleaned_dir`).
    - O `cleaner-templates.json` é lido para carregar o mapa de ferramentas e seus respectivos caminhos de arquivo de template.

2.  **Identificação da Ferramenta**:
    - O nome do arquivo de entrada (`filename`, ex: `nmap_example.com_12345.txt`) é inspecionado.
    - O script procura por um prefixo que corresponda a uma das chaves no mapa de `TemplatePaths` (ex: `nmap_`).
    - Uma vez encontrado, o nome da ferramenta (`toolName`) e o caminho para seu arquivo de template (`templateFilePath`) são armazenados.

3.  **Carregamento do Template Específico**:
    - O arquivo de template da ferramenta identificada (ex: `/etc/AutoHunting/templates/cleaner/nmap.json`) é lido.
    - O conteúdo JSON é decodificado na `struct` `ToolTemplates`.
    - Usando o `templateName` fornecido na chamada da função (ex: `"open_ports"`), a regra de limpeza específica (`selectedTemplate`) é selecionada do mapa.

4.  **Processamento do Arquivo Bruto**:
    *   O arquivo de resultado bruto é aberto e lido linha por linha.
    *   A expressão regular (`selectedTemplate.Regex`) é compilada para otimização.
    *   Para cada linha do arquivo, o regex compilado é aplicado para encontrar correspondências e extrair os grupos de captura (as partes entre parênteses).

5.  **Formatação da Saída**:
    *   Se uma linha corresponde ao padrão, os dados extraídos pelos grupos de captura são coletados.
    *   Esses dados são unidos em uma única string, com cada campo separado pelo caractere pipe (`|`).
    *   A linha formatada é adicionada a uma lista de resultados limpos.

6.  **Geração do Arquivo de Saída**:
    *   Um novo nome de arquivo é gerado, adicionando o sufixo `_clean_[templateName].txt` ao nome do arquivo original.
    *   O diretório de saída (`tool_cleaned_dir`) é verificado e criado, se necessário.
    *   O novo arquivo é criado no caminho de saída completo.
    *   Todas as linhas formatadas são escritas no novo arquivo, uma por uma.

### Exemplo Prático

- **Entrada**:
    - `filename`: `"results/dirt/nmap_example.com_12345.txt"`
    - `templateName`: `"open_ports"`
    - Conteúdo do arquivo de entrada:
      ```
      ...
      PORT      STATE SERVICE
      80/tcp    open  http
      443/tcp   open  https
      8080/tcp  closed http-proxy
      ...
      ```

- **Configuração (`env.json`)**:
  ```json
  {
    "path": {
      "tool_cleaned_dir": "/etc/AutoHunting/Clean/"
    }
  }
