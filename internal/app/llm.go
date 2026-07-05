package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func generateScript(cfg Config, prompt string, similar []Script) (string, error) {
	request := buildGenerationPrompt(prompt, similar)
	switch strings.ToLower(cfg.LLMProvider) {
	case "custom":
		return runCustomProvider(cfg, request)
	case "openai", "anthropic", "ollama", "lm-studio", "lmstudio", "openai-compatible", "":
		return runOpenAICompatible(cfg, request)
	default:
		return "", fmt.Errorf("unsupported llm provider: %s", cfg.LLMProvider)
	}
}

func buildGenerationPrompt(prompt string, similar []Script) string {
	var b strings.Builder
	b.WriteString("Create exactly one local utility script. Return only the script text, no markdown fences.\n\n")
	b.WriteString(scriptShapeGuidance())
	b.WriteString("\n\nPrefer bash for simple local automation. Include --help. Include --dry-run for risky operations. Use conservative defaults and clear errors.\n\n")
	if len(similar) > 0 {
		b.WriteString("Similar existing scripts:\n")
		for _, script := range similar {
			fmt.Fprintf(&b, "- %s: %s\n", script.Metadata.Command, script.Metadata.Description)
		}
		b.WriteString("\n")
	}
	b.WriteString("User request:\n")
	b.WriteString(prompt)
	b.WriteByte('\n')
	return b.String()
}

func runCustomProvider(cfg Config, prompt string) (string, error) {
	if cfg.LLMCustomCmd == "" {
		return "", errors.New("custom provider requires llm_custom_command")
	}
	cmd := exec.Command("sh", "-c", cfg.LLMCustomCmd)
	cmd.Stdin = strings.NewReader(prompt)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%w: %s", err, stderr.String())
		}
		return "", err
	}
	return stripMarkdownFences(out.String()), nil
}

func runOpenAICompatible(cfg Config, prompt string) (string, error) {
	if cfg.LLMBaseURL == "" {
		return "", errors.New("llm_base_url is not configured")
	}
	body := map[string]any{
		"model": cfg.LLMModel,
		"messages": []map[string]string{
			{"role": "system", "content": "You generate small, safe, reviewable local utility scripts."},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.2,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(cfg.LLMBaseURL, "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.LLMAPIKeyEnv != "" {
		if key := os.Getenv(cfg.LLMAPIKeyEnv); key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("llm provider returned %s: %s", resp.Status, string(respBody))
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", err
	}
	if len(decoded.Choices) == 0 || decoded.Choices[0].Message.Content == "" {
		return "", errors.New("llm provider returned no script content")
	}
	return stripMarkdownFences(decoded.Choices[0].Message.Content), nil
}

func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s + "\n"
	}
	lines := strings.Split(s, "\n")
	if len(lines) >= 2 {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n")) + "\n"
}
