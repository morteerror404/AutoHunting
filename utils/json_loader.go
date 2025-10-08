package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Estruturas para a ordem de execução
type MaestroTask struct {
	Step        string `json:"step"`
	Description string `json:"description"`
}

type MaestroOrder struct {
	Platform string            `json:"platform"`
	Task     string            `json:"task"`
	Steps    []MaestroTask     `json:"steps"`
	Data     map[string]string `json:"data,omitempty"`
}

// LoadJSON carrega um arquivo JSON do diretório json/ e decodifica para a estrutura fornecida
func LoadJSON(filename string, v interface{}) error {
	absPath := filepath.Join("json", filename)

	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("erro ao ler arquivo %s: %w", absPath, err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("erro ao decodificar JSON de %s: %w", absPath, err)
	}

	return nil
}

// CreateExecutionOrder gera o arquivo de ordem para o maestro.
func CreateExecutionOrder(task, platform string, data map[string]string) error {
	// 1. Carregar env.json para obter os caminhos
	var envConfig struct {
		Archives struct {
			MaestroExecOrder      string `json:"maestro_exec_order"`
			MaestroOrderTemplates string `json:"maestro_order_templates"`
		} `json:"archives"`
	}
	if err := LoadJSON("env.json", &envConfig); err != nil {
		return fmt.Errorf("erro ao carregar env.json para criar ordem: %w", err)
	}

	orderTemplatesPath := envConfig.Archives.MaestroOrderTemplates
	maestroOrderPath := envConfig.Archives.MaestroExecOrder

	// 2. Carregar o arquivo de templates de ordem
	var orderTemplates struct {
		ExecutionPlans map[string][]MaestroTask `json:"execution_plans"`
	}
	// Usamos LoadJSON com um caminho relativo, pois ele já adiciona o prefixo "json/"
	// Precisamos extrair o nome do arquivo do caminho absoluto.
	templateFilename := filepath.Base(orderTemplatesPath)
	if err := LoadJSON(templateFilename, &orderTemplates); err != nil {
		return fmt.Errorf("erro ao carregar o template de ordens '%s': %w", orderTemplatesPath, err)
	}

	// 3. Encontrar o plano de execução para a tarefa solicitada
	steps, ok := orderTemplates.ExecutionPlans[task]
	if !ok {
		return fmt.Errorf("plano de execução para a tarefa '%s' não encontrado em '%s'", task, orderTemplatesPath)
	}

	// 4. Montar a ordem final
	finalOrder := MaestroOrder{
		Platform: platform,
		Task:     task,
		Steps:    steps,
		Data:     data,
	}

	// 5. Serializar e salvar o arquivo de ordem final para o maestro
	orderData, err := json.MarshalIndent(finalOrder, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao serializar a ordem final do maestro: %w", err)
	}

	// Garante que o diretório de destino exista
	if err := os.MkdirAll(filepath.Dir(maestroOrderPath), 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório para a ordem do maestro: %w", err)
	}

	if err := ioutil.WriteFile(maestroOrderPath, orderData, 0644); err != nil {
		return fmt.Errorf("erro ao escrever o arquivo de ordem do maestro '%s': %w", maestroOrderPath, err)
	}

	fmt.Printf("Ordem de execução para a tarefa '%s' criada com sucesso em: %s\n", task, maestroOrderPath)
	return nil
}
