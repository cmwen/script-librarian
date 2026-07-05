package app

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}

func cmdMCP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "tools" {
		for _, name := range mcpToolNames() {
			fmt.Fprintln(stdout, name)
		}
		return 0
	}
	if len(args) > 0 && args[0] == "instructions" {
		fmt.Fprintln(stdout, agentInstructions())
		return 0
	}
	cfg, _, err := requireConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := serveMCP(cfg, stdin, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func serveMCP(cfg Config, r io.Reader, w io.Writer) error {
	reader := bufio.NewReader(r)
	for {
		payload, err := readMCPMessage(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		var req rpcRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			return err
		}
		resp := handleMCPRequest(cfg, req)
		if resp == nil {
			continue
		}
		encoded, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(encoded), encoded); err != nil {
			return err
		}
	}
}

func readMCPMessage(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(line, "Content-Length:") {
		lengthText := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
		length, err := strconv.Atoi(lengthText)
		if err != nil {
			return nil, err
		}
		for {
			header, err := r.ReadString('\n')
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(header) == "" {
				break
			}
		}
		payload := make([]byte, length)
		_, err = io.ReadFull(r, payload)
		return payload, err
	}
	var buf bytes.Buffer
	buf.WriteString(line)
	return bytes.TrimSpace(buf.Bytes()), nil
}

func handleMCPRequest(cfg Config, req rpcRequest) *rpcResponse {
	resp := &rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "script-librarian",
				"version": "dev",
			},
		}
	case "notifications/initialized":
		return nil
	case "tools/list":
		resp.Result = map[string]any{"tools": mcpTools()}
	case "tools/call":
		result, err := callMCPTool(cfg, req.Params)
		if err != nil {
			resp.Error = map[string]any{"code": -32000, "message": err.Error()}
		} else {
			resp.Result = map[string]any{
				"content": []map[string]string{{"type": "text", "text": result}},
			}
		}
	default:
		resp.Error = map[string]any{"code": -32601, "message": "method not found"}
	}
	return resp
}

func mcpToolNames() []string {
	return []string{"list_scripts", "search_scripts", "get_script", "get_script_shape", "validate_script", "save_script", "update_script"}
}

func mcpTools() []map[string]any {
	var tools []map[string]any
	for _, name := range mcpToolNames() {
		tools = append(tools, map[string]any{
			"name":        name,
			"description": mcpDescription(name),
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
		})
	}
	return tools
}

func mcpDescription(name string) string {
	switch name {
	case "list_scripts":
		return "List managed scripts."
	case "search_scripts":
		return "Search managed scripts by command, description, tags, aliases, usage, and examples."
	case "get_script":
		return "Get script metadata and source path."
	case "get_script_shape":
		return "Return the required Script Librarian metadata shape."
	case "validate_script":
		return "Validate script text against the required metadata shape."
	case "save_script":
		return "Save a new managed script after validation."
	case "update_script":
		return "Replace an existing managed script after validation."
	default:
		return name
	}
}

func callMCPTool(cfg Config, raw json.RawMessage) (string, error) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return "", err
	}
	switch params.Name {
	case "list_scripts":
		scripts, err := scanScripts(cfg.ScriptsDir)
		if err != nil {
			return "", err
		}
		return scriptsAsJSON(scripts)
	case "search_scripts":
		query, _ := params.Arguments["query"].(string)
		scripts, err := scanScripts(cfg.ScriptsDir)
		if err != nil {
			return "", err
		}
		return scriptsAsJSON(searchScripts(scripts, query))
	case "get_script":
		name, _ := params.Arguments["name"].(string)
		script, err := findScript(cfg.ScriptsDir, name)
		if err != nil {
			return "", err
		}
		return scriptAsJSON(script)
	case "get_script_shape":
		return scriptShapeGuidance(), nil
	case "validate_script":
		text, _ := params.Arguments["text"].(string)
		meta, err := parseMetadata(strings.NewReader(text), "")
		if err != nil {
			return "", err
		}
		result := validateMetadata(meta, true)
		data, _ := json.MarshalIndent(result, "", "  ")
		return string(data), nil
	case "save_script":
		text, _ := params.Arguments["text"].(string)
		script, err := saveScriptContent(cfg.ScriptsDir, text)
		if err != nil {
			return "", err
		}
		if cfg.GitEnabled && cfg.AutoCommit {
			if err := gitCommitFile(cfg.ScriptsDir, script.RelPath, "Add "+script.Metadata.Command+" script"); err != nil {
				return "", err
			}
		}
		if cfg.ShimDir != "" {
			if err := ensureShim(cfg.ShimDir, script); err != nil {
				return "", err
			}
		}
		return scriptAsJSON(script)
	case "update_script":
		text, _ := params.Arguments["text"].(string)
		name, _ := params.Arguments["name"].(string)
		existing, err := findScript(cfg.ScriptsDir, name)
		if err != nil {
			return "", err
		}
		meta, err := parseMetadata(strings.NewReader(text), "")
		if err != nil {
			return "", err
		}
		result := validateMetadata(meta, true)
		if !result.OK {
			return "", errors.New(strings.Join(result.Errors, "; "))
		}
		if err := writeExecutable(existing.Path, text); err != nil {
			return "", err
		}
		if cfg.GitEnabled && cfg.AutoCommit {
			if err := gitCommitFile(cfg.ScriptsDir, existing.RelPath, "Update "+existing.Metadata.Command+" script"); err != nil {
				return "", err
			}
		}
		updated, err := findScript(cfg.ScriptsDir, meta.Command)
		if err != nil {
			return "", err
		}
		if cfg.ShimDir != "" {
			if err := ensureShim(cfg.ShimDir, updated); err != nil {
				return "", err
			}
		}
		return scriptAsJSON(updated)
	default:
		return "", fmt.Errorf("unknown MCP tool: %s", params.Name)
	}
}

func scriptsAsJSON(scripts []Script) (string, error) {
	type item struct {
		Command     string   `json:"command"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Usage       string   `json:"usage"`
		Tags        []string `json:"tags"`
		Safety      string   `json:"safety"`
		Path        string   `json:"path"`
	}
	items := make([]item, 0, len(scripts))
	for _, script := range scripts {
		items = append(items, item{
			Command:     script.Metadata.Command,
			Name:        script.Metadata.Name,
			Description: script.Metadata.Description,
			Usage:       script.Metadata.Usage,
			Tags:        script.Metadata.Tags,
			Safety:      script.Metadata.Safety,
			Path:        script.Path,
		})
	}
	data, err := json.MarshalIndent(items, "", "  ")
	return string(data), err
}

func scriptAsJSON(script Script) (string, error) {
	data, err := json.MarshalIndent(map[string]any{
		"command":      script.Metadata.Command,
		"name":         script.Metadata.Name,
		"description":  script.Metadata.Description,
		"usage":        script.Metadata.Usage,
		"tags":         script.Metadata.Tags,
		"aliases":      script.Metadata.Aliases,
		"runtime":      script.Metadata.Runtime,
		"safety":       script.Metadata.Safety,
		"dependencies": script.Metadata.Dependencies,
		"examples":     script.Metadata.Examples,
		"path":         script.Path,
	}, "", "  ")
	return string(data), err
}
