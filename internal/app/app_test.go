package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestParseMetadataAndValidation(t *testing.T) {
	script := `#!/usr/bin/env bash
# msl:name Large Files
# msl:description Find large files below a path.
# msl:usage large-files [--min-size 100M] [path]
# msl:tags files, disk, cleanup
# msl:runtime bash
# msl:safety read-only
# msl:deps find, du
# msl:example large-files --min-size 50M .

echo ok
`
	meta, err := parseMetadata(strings.NewReader(script), "large-files")
	if err != nil {
		t.Fatal(err)
	}
	if meta.Command != "large-files" {
		t.Fatalf("command = %q", meta.Command)
	}
	if got := strings.Join(meta.Dependencies, ","); got != "find,du" {
		t.Fatalf("dependencies = %q", got)
	}
	if result := validateMetadata(meta, true); !result.OK {
		t.Fatalf("validation failed: %#v", result)
	}
}

func TestCLIInitSearchInfoRunAndValidate(t *testing.T) {
	env := newTestEnv(t)
	scriptsDir := filepath.Join(env.root, "scripts")

	out, errOut, code := runForTest(t, []string{"init", "--dir", scriptsDir, "--no-git", "--yes"}, "")
	if code != 0 {
		t.Fatalf("init failed code=%d stdout=%q stderr=%q", code, out, errOut)
	}
	if !strings.Contains(out, "Script library ready") {
		t.Fatalf("init output = %q", out)
	}

	scriptPath := filepath.Join(scriptsDir, "hello")
	writeTestScript(t, scriptPath, `#!/usr/bin/env bash
# msl:name Hello
# msl:description Print a friendly greeting.
# msl:usage hello [name]
# msl:tags demo, greeting
# msl:runtime bash
# msl:safety read-only
# msl:deps bash
# msl:example hello Codex

name="${1:-world}"
printf 'hello %s\n' "$name"
`)

	out, errOut, code = runForTest(t, []string{"search", "greeting"}, "")
	if code != 0 {
		t.Fatalf("search failed code=%d stdout=%q stderr=%q", code, out, errOut)
	}
	if !strings.Contains(out, "hello") || !strings.Contains(out, "friendly greeting") {
		t.Fatalf("search output = %q", out)
	}

	out, errOut, code = runForTest(t, []string{"info", "hello"}, "")
	if code != 0 {
		t.Fatalf("info failed code=%d stdout=%q stderr=%q", code, out, errOut)
	}
	if !strings.Contains(out, "Safety: read-only") || !strings.Contains(out, scriptPath) {
		t.Fatalf("info output = %q", out)
	}

	out, errOut, code = runForTest(t, []string{"run", "hello", "--", "Codex"}, "")
	if code != 0 {
		t.Fatalf("run failed code=%d stdout=%q stderr=%q", code, out, errOut)
	}
	if strings.TrimSpace(out) != "hello Codex" {
		t.Fatalf("run output = %q", out)
	}

	out, errOut, code = runForTest(t, []string{"validate", scriptPath}, "")
	if code != 0 {
		t.Fatalf("validate failed code=%d stdout=%q stderr=%q", code, out, errOut)
	}
	if !strings.Contains(out, "Validation passed") {
		t.Fatalf("validate output = %q", out)
	}
}

func TestNewWithCustomProvider(t *testing.T) {
	env := newTestEnv(t)
	scriptsDir := filepath.Join(env.root, "scripts")
	out, errOut, code := runForTest(t, []string{"init", "--dir", scriptsDir, "--no-git", "--yes"}, "")
	if code != 0 {
		t.Fatalf("init failed code=%d stdout=%q stderr=%q", code, out, errOut)
	}

	provider := filepath.Join(env.root, "provider.sh")
	writeFile(t, provider, `#!/usr/bin/env bash
cat <<'SCRIPT'
#!/usr/bin/env bash
# msl:name Say Hi
# msl:description Say hi to the current user.
# msl:usage say-hi
# msl:tags demo, greeting
# msl:runtime bash
# msl:safety read-only
# msl:deps bash
# msl:example say-hi

printf 'hi\n'
SCRIPT
`, 0o755)
	t.Setenv(customCommandEnv, provider)

	out, errOut, code = runForTest(t, []string{"new", "--yes", "make a hello script"}, "")
	if code != 0 {
		t.Fatalf("new failed code=%d stdout=%q stderr=%q", code, out, errOut)
	}
	if !strings.Contains(out, "Saved:") {
		t.Fatalf("new output = %q", out)
	}
	if _, err := os.Stat(filepath.Join(scriptsDir, "say-hi")); err != nil {
		t.Fatalf("generated script not saved: %v", err)
	}
}

func TestMCPFramedToolsListAndSearch(t *testing.T) {
	env := newTestEnv(t)
	scriptsDir := filepath.Join(env.root, "scripts")
	_, _, code := runForTest(t, []string{"init", "--dir", scriptsDir, "--no-git", "--yes"}, "")
	if code != 0 {
		t.Fatal("init failed")
	}
	writeTestScript(t, filepath.Join(scriptsDir, "hello"), `#!/usr/bin/env bash
# msl:name Hello
# msl:description Print a friendly greeting.
# msl:usage hello
# msl:tags demo, greeting
# msl:runtime bash
# msl:safety read-only
# msl:deps bash
# msl:example hello

printf 'hello\n'
`)

	cfg, _, err := requireConfig()
	if err != nil {
		t.Fatal(err)
	}
	input := mcpFrame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		mcpFrame(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`) +
		mcpFrame(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_scripts","arguments":{"query":"greeting"}}}`)
	var out bytes.Buffer
	if err := serveMCP(cfg, strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "search_scripts") {
		t.Fatalf("tools/list response = %q", got)
	}
	if !strings.Contains(got, "hello") {
		t.Fatalf("search response = %q", got)
	}
}

func TestSplitArgsSupportsQuotedSpacing(t *testing.T) {
	args, err := splitArgs(`--name "Ada Lovelace" --path 'one two' escaped\ space`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--name", "Ada Lovelace", "--path", "one two", "escaped space"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
	if _, err := splitArgs(`"unterminated`); err == nil {
		t.Fatal("expected unterminated quote error")
	}
}

func TestFinderTypingFiltersAndEnterOpensArgs(t *testing.T) {
	scripts := []Script{
		{Path: "/tmp/hello", Metadata: Metadata{Command: "hello", Name: "Hello", Description: "Print greeting", Usage: "hello [name]", Runtime: "bash", Safety: "read-only", Tags: []string{"demo"}}},
		{Path: "/tmp/cleanup", Metadata: Metadata{Command: "cleanup", Name: "Cleanup", Description: "Remove branches", Usage: "cleanup", Runtime: "bash", Safety: "writes-project", Tags: []string{"git"}}},
	}
	model := newFinderModel(Config{ScriptsDir: "/tmp/scripts"}, scripts)
	updated, _ := model.Update(keyMsg("p"))
	model = updated.(finderModel)
	updated, _ = model.Update(keyMsg("r"))
	model = updated.(finderModel)
	if len(model.filtered) != 1 || model.filtered[0].Metadata.Command != "hello" {
		t.Fatalf("filtered = %#v", model.filtered)
	}
	updated, _ = model.Update(keyMsg("enter"))
	model = updated.(finderModel)
	if model.mode != modeArgs {
		t.Fatalf("mode = %v", model.mode)
	}
}

type testEnv struct {
	root string
}

func newTestEnv(t *testing.T) testEnv {
	t.Helper()
	root := t.TempDir()
	t.Setenv(configEnv, filepath.Join(root, "config.toml"))
	t.Setenv(scriptsDirEnv, "")
	t.Setenv(customCommandEnv, "")
	return testEnv{root: root}
}

func runForTest(t *testing.T, args []string, input string) (string, string, int) {
	t.Helper()
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(args, strings.NewReader(input), &out, &errOut)
	return out.String(), errOut.String(), code
}

func writeTestScript(t *testing.T, path, content string) {
	t.Helper()
	writeFile(t, path, content, 0o755)
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func mcpFrame(payload string) string {
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(payload), payload)
}

func keyMsg(s string) tea.KeyMsg {
	if len([]rune(s)) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}
