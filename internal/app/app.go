package app

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return runPicker(stdin, stdout, stderr)
	}
	switch args[0] {
	case "init":
		return cmdInit(args[1:], stdin, stdout, stderr)
	case "search":
		return cmdSearch(args[1:], stdout, stderr)
	case "info":
		return cmdInfo(args[1:], stdout, stderr)
	case "run":
		return cmdRun(args[1:], stdin, stdout, stderr)
	case "new":
		return cmdNew(args[1:], stdin, stdout, stderr)
	case "validate":
		return cmdValidate(args[1:], stdout, stderr)
	case "llm":
		return cmdLLM(args[1:], stdin, stdout, stderr)
	case "scripts":
		return cmdScripts(args[1:], stdout, stderr)
	case "mcp":
		return cmdMCP(args[1:], stdin, stdout, stderr)
	case "help", "-h", "--help":
		printHelp(stdout)
		return 0
	case "version", "--version":
		fmt.Fprintln(stdout, "msl dev")
		return 0
	default:
		if code, ok := cmdShortcutRun(args, stdin, stdout, stderr); ok {
			return code
		}
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		printHelp(stderr)
		return 2
	}
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, `Script Librarian

Usage:
  msl
  msl init [--dir PATH] [--no-git] [--yes]
  msl search <query>
  msl info <script>
  msl run <script> [--dry-run] [--yes] -- [args]
  msl <script> [args]
  msl new [--yes] <prompt>
  msl validate <file>
  msl scripts status|log|diff [script]
  msl scripts shims
  msl llm configure [flags]
  msl mcp
  msl mcp instructions
`)
}

func cmdShortcutRun(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, bool) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return 0, false
	}
	cfg, _, err := requireConfig()
	if err != nil {
		return 0, false
	}
	script, err := findScript(cfg.ScriptsDir, args[0])
	if err != nil {
		return 0, false
	}
	return executeScript(script, args[1:], false, false, stdin, stdout, stderr), true
}

func runPicker(stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, _, err := requireConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if inFile, inOK := stdin.(*os.File); inOK && isTerminal(inFile) {
		if outFile, outOK := stdout.(*os.File); outOK && isTerminal(outFile) {
			return runFinder(cfg, inFile, outFile, stderr)
		}
	}
	query := ""
	if f, ok := stdin.(*os.File); ok && isTerminal(f) {
		fmt.Fprint(stdout, "Search scripts\n> ")
		line, _ := bufio.NewReader(stdin).ReadString('\n')
		query = strings.TrimSpace(line)
	}
	scripts, err := scanScripts(cfg.ScriptsDir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printScriptList(stdout, searchScripts(scripts, query))
	return 0
}

func cmdInit(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg := defaultConfig()
	dirDefault, _ := defaultScriptsDir()
	cfg.ScriptsDir = dirDefault
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", cfg.ScriptsDir, "managed script directory")
	noGit := fs.Bool("no-git", false, "skip git initialization")
	shimDir := fs.String("shims", "", "optional shim directory to create")
	yes := fs.Bool("yes", false, "accept defaults without prompting")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg.ScriptsDir, _ = expandPath(*dir)
	cfg.GitEnabled = !*noGit
	cfg.ShimDir, _ = expandPath(*shimDir)
	if !*yes && isInteractive(stdin) {
		cfg = promptInit(cfg, stdin, stdout)
	}
	if err := os.MkdirAll(cfg.ScriptsDir, 0o755); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if cfg.GitEnabled {
		if err := gitInit(cfg.ScriptsDir); err != nil {
			fmt.Fprintf(stderr, "git init failed: %v\n", err)
			return 1
		}
	}
	if cfg.ShimDir != "" {
		if err := os.MkdirAll(cfg.ShimDir, 0o755); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		count, err := syncShims(cfg)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if count > 0 {
			fmt.Fprintf(stdout, "Created %d script shims.\n", count)
		}
	}
	cfgPath, err := saveConfig(cfg)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "Script library ready:")
	fmt.Fprintf(stdout, "  Scripts: %s\n", cfg.ScriptsDir)
	fmt.Fprintf(stdout, "  Git: %t\n", cfg.GitEnabled)
	if cfg.ShimDir != "" {
		fmt.Fprintf(stdout, "  Shims: %s\n", cfg.ShimDir)
	}
	fmt.Fprintf(stdout, "  Config: %s\n", cfgPath)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Run `msl new \"...\"` to create a script.")
	fmt.Fprintln(stdout, "Run `msl` to search your library.")
	fmt.Fprintln(stdout, "Run `msl mcp instructions` to print reusable agent instructions.")
	return 0
}

func promptInit(cfg Config, stdin io.Reader, stdout io.Writer) Config {
	reader := bufio.NewReader(stdin)
	fmt.Fprintf(stdout, "Where should managed scripts live?\n> %s\n", cfg.ScriptsDir)
	fmt.Fprint(stdout, "Initialize git history there? [Y/n]\n> ")
	answer, _ := reader.ReadString('\n')
	if strings.EqualFold(strings.TrimSpace(answer), "n") {
		cfg.GitEnabled = false
	}
	return cfg
}

func cmdSearch(args []string, stdout, stderr io.Writer) int {
	cfg, _, err := requireConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	scripts, err := scanScripts(cfg.ScriptsDir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printScriptList(stdout, searchScripts(scripts, strings.Join(args, " ")))
	return 0
}

func cmdInfo(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: msl info <script>")
		return 2
	}
	cfg, _, err := requireConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	script, err := findScript(cfg.ScriptsDir, args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printScriptInfo(stdout, script)
	return 0
}

func cmdRun(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dryRun := fs.Bool("dry-run", false, "print the command without executing it")
	yes := fs.Bool("yes", false, "skip confirmation for sensitive scripts")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) < 1 {
		fmt.Fprintln(stderr, "usage: msl run <script> [--dry-run] [--yes] -- [args]")
		return 2
	}
	name := rest[0]
	scriptArgs := rest[1:]
	if len(scriptArgs) > 0 && scriptArgs[0] == "--" {
		scriptArgs = scriptArgs[1:]
	}
	cfg, _, err := requireConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	script, err := findScript(cfg.ScriptsDir, name)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return executeScript(script, scriptArgs, *dryRun, *yes, stdin, stdout, stderr)
}

func cmdNew(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.SetOutput(stderr)
	yes := fs.Bool("yes", false, "save without an interactive prompt after validation")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	prompt := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if prompt == "" {
		fmt.Fprintln(stderr, "usage: msl new [--yes] <prompt>")
		return 2
	}
	cfg, _, err := requireConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	scripts, err := scanScripts(cfg.ScriptsDir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	similar := searchScripts(scripts, prompt)
	if len(similar) > 3 {
		similar = similar[:3]
	}
	if len(similar) > 0 {
		fmt.Fprintln(stdout, "Similar scripts:")
		printScriptList(stdout, similar)
		fmt.Fprintln(stdout)
	}
	content, err := generateScript(cfg, prompt, similar)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	meta, err := parseMetadata(strings.NewReader(content), "")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	validation := validateMetadata(meta, true)
	if !validation.OK {
		for _, issue := range validation.Errors {
			fmt.Fprintln(stderr, issue)
		}
		return 1
	}
	if unsafeMismatch(meta, content) {
		fmt.Fprintln(stderr, "script appears unsafe for its declared safety level")
		return 1
	}
	fmt.Fprintln(stdout, "Review generated script")
	printAddedDiff(stdout, content)
	if !*yes && !confirm(stdin, stdout, "Save this script?") {
		fmt.Fprintln(stdout, "Canceled.")
		return 1
	}
	path := filepath.Join(cfg.ScriptsDir, meta.Command)
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(stderr, "script already exists: %s\n", path)
		return 1
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if cfg.ShimDir != "" {
		script, err := findScript(cfg.ScriptsDir, meta.Command)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := ensureShim(cfg.ShimDir, script); err != nil {
			fmt.Fprintf(stderr, "saved but shim creation failed: %v\n", err)
			return 1
		}
	}
	if cfg.GitEnabled && cfg.AutoCommit {
		if err := gitCommitFile(cfg.ScriptsDir, meta.Command, "Add "+meta.Command+" script"); err != nil {
			fmt.Fprintf(stderr, "saved but git commit failed: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Committed: Add %s script\n", meta.Command)
	}
	fmt.Fprintf(stdout, "Saved: %s\n", path)
	fmt.Fprintln(stdout, "Made executable.")
	return 0
}

func cmdValidate(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: msl validate <file>")
		return 2
	}
	meta, err := parseScriptFile(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	result := validateMetadata(meta, true)
	if result.OK {
		fmt.Fprintln(stdout, "Validation passed.")
		return 0
	}
	for _, issue := range result.Errors {
		fmt.Fprintln(stderr, issue)
	}
	return 1
}

func cmdLLM(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "configure" {
		fmt.Fprintln(stderr, "usage: msl llm configure [--provider custom|openai-compatible] [--base-url URL] [--model MODEL] [--custom-command CMD]")
		return 2
	}
	cfg, path, err := loadConfig()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(stderr, err)
		return 1
	}
	_ = path
	fs := flag.NewFlagSet("llm configure", flag.ContinueOnError)
	fs.SetOutput(stderr)
	provider := fs.String("provider", cfg.LLMProvider, "provider name")
	baseURL := fs.String("base-url", cfg.LLMBaseURL, "OpenAI-compatible base URL")
	model := fs.String("model", cfg.LLMModel, "model name")
	apiKeyEnv := fs.String("api-key-env", cfg.LLMAPIKeyEnv, "environment variable for API key")
	customCmd := fs.String("custom-command", cfg.LLMCustomCmd, "custom command that writes a script to stdout")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if cfg.ScriptsDir == "" {
		cfg.ScriptsDir, _ = defaultScriptsDir()
	}
	cfg.LLMProvider = *provider
	cfg.LLMBaseURL = strings.TrimRight(*baseURL, "/")
	cfg.LLMModel = *model
	cfg.LLMAPIKeyEnv = *apiKeyEnv
	cfg.LLMCustomCmd = *customCmd
	cfgPath, err := saveConfig(cfg)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "LLM provider configured: %s\n", cfg.LLMProvider)
	fmt.Fprintf(stdout, "Config: %s\n", cfgPath)
	return 0
}

func cmdScripts(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: msl scripts status|log|diff [script]")
		return 2
	}
	cfg, _, err := requireConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch args[0] {
	case "status":
		out, err := gitRun(cfg.ScriptsDir, "status", "--short")
		if err != nil {
			fmt.Fprint(stderr, out)
			return 1
		}
		fmt.Fprint(stdout, out)
	case "log":
		gitArgs := []string{"log", "--oneline", "--decorate"}
		if len(args) > 1 {
			script, err := findScript(cfg.ScriptsDir, args[1])
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			gitArgs = append(gitArgs, "--", script.RelPath)
		}
		out, err := gitRun(cfg.ScriptsDir, gitArgs...)
		if err != nil {
			fmt.Fprint(stderr, out)
			return 1
		}
		fmt.Fprint(stdout, out)
	case "diff":
		gitArgs := []string{"diff"}
		if len(args) > 1 {
			script, err := findScript(cfg.ScriptsDir, args[1])
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			gitArgs = append(gitArgs, "--", script.RelPath)
		}
		out, err := gitRun(cfg.ScriptsDir, gitArgs...)
		if err != nil {
			fmt.Fprint(stderr, out)
			return 1
		}
		fmt.Fprint(stdout, out)
	case "shims":
		count, err := syncShims(cfg)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "Synced %d shims.\n", count)
	default:
		fmt.Fprintf(stderr, "unknown scripts command: %s\n", args[0])
		return 2
	}
	return 0
}

func requireConfig() (Config, string, error) {
	cfg, path, err := loadConfig()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, path, fmt.Errorf("Script Librarian is not initialized. Run `msl init` first. Missing config: %s", path)
		}
		return cfg, path, err
	}
	if cfg.ScriptsDir == "" {
		return cfg, path, fmt.Errorf("scripts_dir is missing in config: %s", path)
	}
	return cfg, path, nil
}

func isInteractive(r io.Reader) bool {
	f, ok := r.(*os.File)
	return ok && isTerminal(f)
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}

func confirm(stdin io.Reader, stdout io.Writer, question string) bool {
	fmt.Fprintf(stdout, "%s [y/N]\n> ", question)
	line, _ := bufio.NewReader(stdin).ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}

func requiresConfirmation(safety string) bool {
	return safety == "destructive" || safety == "requires-confirmation"
}

func executeScript(script Script, scriptArgs []string, dryRun bool, yes bool, stdin io.Reader, stdout, stderr io.Writer) int {
	if dryRun {
		fmt.Fprintf(stdout, "%s %s\n", script.Path, strings.Join(quoteArgs(scriptArgs), " "))
		return 0
	}
	if requiresConfirmation(script.Metadata.Safety) && !yes {
		if !confirm(stdin, stdout, fmt.Sprintf("Run %s with safety %s?", script.Metadata.Command, script.Metadata.Safety)) {
			fmt.Fprintln(stdout, "Canceled.")
			return 1
		}
	}
	cmd := exec.Command(script.Path, scriptArgs...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return exitCode(err)
	}
	return 0
}

func printAddedDiff(w io.Writer, content string) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		fmt.Fprintf(w, "+ %s\n", scanner.Text())
	}
}

func unsafeMismatch(meta Metadata, content string) bool {
	lower := strings.ToLower(content)
	risky := []string{" rm -rf ", "mkfs", "dd if=", "sudo rm", "chmod -r", "curl ", "wget "}
	for _, token := range risky {
		if strings.Contains(lower, token) && meta.Safety == "read-only" {
			return true
		}
	}
	return false
}

func exitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

func saveScriptContent(dir string, content string) (Script, error) {
	meta, err := parseMetadata(strings.NewReader(content), "")
	if err != nil {
		return Script{}, err
	}
	validation := validateMetadata(meta, true)
	if !validation.OK {
		return Script{}, errors.New(strings.Join(validation.Errors, "; "))
	}
	path := filepath.Join(dir, meta.Command)
	if _, err := os.Stat(path); err == nil {
		return Script{}, fmt.Errorf("script already exists: %s", meta.Command)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return Script{}, err
	}
	parsed, err := parseScriptFile(path)
	if err != nil {
		return Script{}, err
	}
	info, _ := os.Stat(path)
	script := Script{Path: path, RelPath: meta.Command, Metadata: parsed, ModTime: info.ModTime(), Size: info.Size()}
	return script, nil
}

func writeExecutable(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o755)
}

func bufferRun(args []string) (string, string, int) {
	var out bytes.Buffer
	var stderr bytes.Buffer
	code := Run(args, strings.NewReader(""), &out, &stderr)
	return out.String(), stderr.String(), code
}
