package main

// Config holds the configuration from env.json
type Config struct {
	Path struct {
		APIDirtResultsPath string `json:"api_dirt_results_path"`
		ToolDirtDir        string `json:"tool_dirt_dir"`
		ToolCleanedDir     string `json:"tool_cleaned_dir"`
	} `json:"path"`
	Archives struct {
		MaestroExecOrder string `json:"maestro_exec_order"`
		LogDir           string `json:"log_dir"`
	} `json:"archives"`
}

// Commands holds the command templates from commands.json
type Commands struct {
	Nmap map[string]string `json:"nmap"`
	Ffuf map[string]string `json:"ffuf"`
}

// Tokens holds the API tokens from tokens.json
type Tokens struct {
	HackerOne struct {
		Username string `json:"username"`
		ApiKey   string `json:"api_key"`
	} `json:"hackerone"`
	Bugcrowd struct {
		Token string `json:"token"`
	} `json:"bugcrowd"`
	Intigriti struct {
		Token string `json:"token"`
	} `json:"intigriti"`
	YesWeHack struct {
		Token string `json:"token"`
	} `json:"yeswehack"`
}
