package app

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func defaultConfig() Config {
	return Config{
		GitEnabled:   true,
		AutoCommit:   true,
		LLMProvider:  "openai-compatible",
		LLMBaseURL:   "http://localhost:11434/v1",
		LLMModel:     "gemma",
		LLMAPIKeyEnv: "OPENAI_API_KEY",
	}
}

func configPath() (string, error) {
	if p := os.Getenv(configEnv); p != "" {
		return expandPath(p)
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, defaultProductDir, "config.toml"), nil
}

func defaultScriptsDir() (string, error) {
	if p := os.Getenv(scriptsDirEnv); p != "" {
		return expandPath(p)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "scripts"), nil
}

func loadConfig() (Config, string, error) {
	cfg := defaultConfig()
	path, err := configPath()
	if err != nil {
		return cfg, "", err
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, path, err
		}
		return cfg, path, err
	}
	defer f.Close()
	if err := parseConfig(f, &cfg); err != nil {
		return cfg, path, err
	}
	if override := os.Getenv(scriptsDirEnv); override != "" {
		cfg.ScriptsDir, _ = expandPath(override)
	}
	if override := os.Getenv(customCommandEnv); override != "" {
		cfg.LLMProvider = "custom"
		cfg.LLMCustomCmd = override
	}
	return cfg, path, nil
}

func parseConfig(r io.Reader, cfg *Config) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"`)
		switch key {
		case "scripts_dir":
			cfg.ScriptsDir, _ = expandPath(val)
		case "git_enabled":
			cfg.GitEnabled = parseBool(val, cfg.GitEnabled)
		case "auto_commit":
			cfg.AutoCommit = parseBool(val, cfg.AutoCommit)
		case "shim_dir":
			cfg.ShimDir, _ = expandPath(val)
		case "llm_provider":
			cfg.LLMProvider = val
		case "llm_base_url":
			cfg.LLMBaseURL = strings.TrimRight(val, "/")
		case "llm_model":
			cfg.LLMModel = val
		case "llm_api_key_env":
			cfg.LLMAPIKeyEnv = val
		case "llm_custom_command":
			cfg.LLMCustomCmd = val
		case "last_configured":
			if t, err := time.Parse(time.RFC3339, val); err == nil {
				cfg.LastConfigured = t
			}
		}
	}
	return scanner.Err()
}

func saveConfig(cfg Config) (string, error) {
	path, err := configPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if cfg.LastConfigured.IsZero() {
		cfg.LastConfigured = time.Now()
	}
	content := fmt.Sprintf(`# Script Librarian local configuration.
scripts_dir = %q
git_enabled = %t
auto_commit = %t
shim_dir = %q
llm_provider = %q
llm_base_url = %q
llm_model = %q
llm_api_key_env = %q
llm_custom_command = %q
last_configured = %q
`, cfg.ScriptsDir, cfg.GitEnabled, cfg.AutoCommit, cfg.ShimDir, cfg.LLMProvider, cfg.LLMBaseURL, cfg.LLMModel, cfg.LLMAPIKeyEnv, cfg.LLMCustomCmd, cfg.LastConfigured.Format(time.RFC3339))
	return path, os.WriteFile(path, []byte(content), 0o600)
}

func parseBool(v string, fallback bool) bool {
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return b
}

func expandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return filepath.Abs(path)
}
