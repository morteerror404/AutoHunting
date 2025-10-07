# Módulo `cleaner` - Inteligência Léxica

## Descrição

O `cleaner.go` é o componente de inteligência léxica do AutoHunting. Sua principal responsabilidade é transformar os resultados brutos e não estruturados, gerados por ferramentas de varredura como Nmap e Nuclei, em dados limpos, formatados e prontos para serem inseridos no banco de dados.

Ele atua como uma ponte entre a fase de execução de ferramentas (`runner`) e a fase de persistência de dados (`db_manager`), garantindo que apenas informações relevantes e bem formatadas sejam armazenadas.

## Funcionalidades Principais

- **Limpeza Baseada em Templates**: Utiliza o arquivo `json/cleaner-templates.json` para definir como os resultados de cada ferramenta devem ser processados. Isso torna o módulo altamente extensível, permitindo adicionar suporte a novas ferramentas sem alterar o código Go.
- **Extração com Expressões Regulares (Regex)**: Aplica padrões de regex para identificar e extrair com precisão os dados importantes (como portas abertas, serviços, URLs vulneráveis) do texto bruto.
- **Formato de Saída Estruturado**: Gera arquivos de saída (`.txt`) onde cada linha representa um achado e os campos são separados por um pipe (`|`). Ex: `80|http`.
- **Nomenclatura de Arquivos Inteligente**: Cria arquivos de saída com nomes descritivos, incorporando o nome da ferramenta, o template usado e um sufixo `_clean_`, facilitando a identificação e o processamento subsequente. Ex: `nmap_example.com_2023_clean_open_ports.txt`.

## Fluxo de Funcionamento

O processo de limpeza é orquestrado pela função `CleanFile`, que segue os seguintes passos:

1.  **Chamada da Função**: O `maestro.go` invoca `cleaner.CleanFile(filename, templateName)`, passando o caminho do arquivo de resultado bruto e o nome do template a ser usado (ex: `open_ports`).

2.  **Carregamento dos Templates**: O `cleaner` carrega o arquivo `json/cleaner-templates.json` para ter acesso a todas as regras de limpeza disponíveis.

3.  **Seleção do Template Correto**:
    *   O nome do arquivo de entrada (ex: `nmap_...` ou `nuclei_...`) é inspecionado para identificar qual ferramenta gerou o resultado.
    *   Com base na ferramenta e no `templateName` fornecido, a regra de limpeza específica (que contém o `regex` e os `fields`) é selecionada.

4.  **Leitura e Processamento do Arquivo Bruto**:
    *   O arquivo de resultado bruto é aberto e lido linha por linha.
    *   A expressão regular (regex) do template selecionado é aplicada a cada linha.

5.  **Extração e Formatação**:
    *   Se uma linha corresponde ao padrão regex, os grupos de captura (as partes entre parênteses no regex) são extraídos.
    *   Os dados extraídos são unidos em uma única string, separados pelo caractere pipe (`|`).

6.  **Geração do Arquivo de Saída**:
    *   Um novo nome de arquivo é gerado, como `[nome_original]_clean_[template].txt`.
    *   Todas as linhas de dados formatados são escritas neste novo arquivo.

### Exemplo Prático

- **Entrada**:
    - `filename`: `"results/dirt/nmap_example.com_12345.txt"`
    - `templateName`: `"open_ports"`
    - Conteúdo do arquivo:
      ```
      ...
      PORT      STATE SERVICE
      80/tcp    open  http
      443/tcp   open  https
      8080/tcp  closed http-proxy
      ...
      ```

- **Template (`cleaner-templates.json`)**:
  ```json
  {
    "nmap": {
      "open_ports": {
        "regex": "^(\d+)/tcp\s+open\s+([^\s]+)",
        "fields": ["port", "service"]
      }
    }
  }
