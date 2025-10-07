# Módulo `utils` - Utilitários Gerais

## Descrição

O pacote `utils` agrupa funções auxiliares e genéricas que são utilizadas por múltiplos módulos ao longo do projeto `AutoHunting`. O arquivo `json_loader.go` é um componente central deste pacote, fornecendo uma maneira padronizada e robusta de carregar e decodificar arquivos de configuração no formato JSON.

Ao centralizar essa lógica, o `json_loader.go` elimina a duplicação de código e garante um tratamento de erros consistente para operações de leitura de arquivos e parsing de JSON.

## Funcionalidades Principais

- **Carregamento Genérico**: Capaz de carregar qualquer arquivo JSON e decodificá-lo em qualquer `struct` Go correspondente, tornando-o extremamente flexível.
- **Tratamento de Erros Centralizado**: Fornece mensagens de erro claras e encapsuladas para falhas ao abrir o arquivo, ler seu conteúdo ou decodificar o JSON.
- **Reusabilidade**: É a função padrão utilizada pelos módulos `maestro`, `db`, `results`, entre outros, para carregar seus respectivos arquivos de configuração (`env.json`, `db_info.json`, `tokens.json`, etc.).

## Descrição das Funções

- **`LoadJSON(filePath string, v interface{}) error`**
  - **Propósito**: Ler um arquivo JSON do disco e popular uma `struct` Go com seus dados.
  - **Funcionamento**:
    1.  Recebe como parâmetros o caminho para o arquivo JSON (`filePath`) e um ponteiro para a `struct` de destino (`v interface{}`).
    2.  Abre o arquivo no caminho especificado. Retorna um erro se o arquivo não for encontrado.
    3.  Lê todo o conteúdo do arquivo em um slice de bytes.
    4.  Utiliza a função `json.Unmarshal` da biblioteca padrão para "mapear" os dados do JSON para os campos da `struct` de destino. As tags de campo (ex: `` `json:"host"` ``) na `struct` guiam esse mapeamento.
    5.  Retorna um erro se o conteúdo do arquivo não for um JSON válido ou se a estrutura não corresponder à `struct` de destino.

## Fluxo de Funcionamento (Exemplo de Uso)

```txt
EXEMPLO DE USO NO MÓDULO `db_manager`

(1) Definição da Struct de Destino
    // No código do db_manager.go, uma struct é definida para espelhar a estrutura do db_info.json
    type DBInfo struct {
        ConfigDB DBConfig                  `json:"config_db"`
        Commands map[string]CommandsConfig `json:"commands"`
    }

(2) Chamada da Função Utilitária
    // Dentro de uma função como db.ConnectDB()
    var dbInfo DBInfo
    err := utils.LoadJSON("db_info.json", &dbInfo) // Passa o nome do arquivo e um ponteiro para a struct
    if err != nil {
        // O erro retornado será detalhado, ex: "erro ao carregar db_info.json: erro ao decodificar JSON: ..."
        return nil, err
    }

(3) Resultado
    -> A variável `dbInfo` agora está preenchida com os dados do arquivo `db_info.json`.
    -> O código pode acessar os dados de forma segura e tipada, como `dbInfo.ConfigDB.Host`.
```