package app

import "time"

const (
	defaultProductDir = "script-librarian"
	configEnv         = "MSL_CONFIG"
	scriptsDirEnv     = "MSL_SCRIPTS_DIR"
	customCommandEnv  = "MSL_LLM_CUSTOM_COMMAND"
)

type Config struct {
	ScriptsDir     string
	GitEnabled     bool
	AutoCommit     bool
	ShimDir        string
	LLMProvider    string
	LLMBaseURL     string
	LLMModel       string
	LLMAPIKeyEnv   string
	LLMCustomCmd   string
	LastConfigured time.Time
}

type Metadata struct {
	Command      string
	Name         string
	Description  string
	Usage        string
	Tags         []string
	Aliases      []string
	Runtime      string
	Safety       string
	Dependencies []string
	Examples     []string
	CreatedBy    string
	UpdatedBy    string
	Raw          map[string][]string
}

type Script struct {
	Path     string
	RelPath  string
	Metadata Metadata
	ModTime  time.Time
	Size     int64
}

type ValidationResult struct {
	OK       bool     `json:"ok"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}
