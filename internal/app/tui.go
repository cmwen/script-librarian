package app

import (
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type finderAction int

const (
	finderNone finderAction = iota
	finderRun
	finderDryRun
	finderInfo
)

type finderMode int

const (
	modeSearch finderMode = iota
	modeArgs
)

type finderResult struct {
	action finderAction
	script Script
	args   []string
}

type finderModel struct {
	cfg      Config
	scripts  []Script
	filtered []Script
	query    string
	cursor   int
	offset   int
	width    int
	height   int
	mode     finderMode
	argInput string
	errText  string
	result   finderResult
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("63")).Bold(true)
	commandStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	safetyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	panelStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1)
	selectedPanel = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Padding(0, 1)
)

func runFinder(cfg Config, stdin, stdout *os.File, stderr io.Writer) int {
	scripts, err := scanScripts(cfg.ScriptsDir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	model := newFinderModel(cfg, scripts)
	program := tea.NewProgram(model, tea.WithInput(stdin), tea.WithOutput(stdout), tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	m, ok := finalModel.(finderModel)
	if !ok {
		return 0
	}
	switch m.result.action {
	case finderRun:
		return executeScript(m.result.script, m.result.args, false, false, stdin, stdout, stderr)
	case finderDryRun:
		return executeScript(m.result.script, m.result.args, true, true, stdin, stdout, stderr)
	case finderInfo:
		printScriptInfo(stdout, m.result.script)
	}
	return 0
}

func newFinderModel(cfg Config, scripts []Script) finderModel {
	m := finderModel{
		cfg:      cfg,
		scripts:  scripts,
		filtered: scripts,
		width:    100,
		height:   30,
	}
	m.applyFilter()
	return m
}

func (m finderModel) Init() tea.Cmd {
	return nil
}

func (m finderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m finderModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch m.mode {
	case modeArgs:
		return m.handleArgKey(key, msg)
	default:
		return m.handleSearchKey(key, msg)
	}
}

func (m finderModel) handleSearchKey(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.errText = ""
	switch key {
	case "ctrl+c", "esc":
		m.result.action = finderNone
		return m, tea.Quit
	case "up", "ctrl+p":
		m.move(-1)
	case "down", "ctrl+n":
		m.move(1)
	case "pgup":
		m.move(-visibleRows(m.height))
	case "pgdown":
		m.move(visibleRows(m.height))
	case "home":
		m.cursor = 0
		m.offset = 0
	case "end":
		m.cursor = len(m.filtered) - 1
		m.ensureCursorVisible()
	case "backspace", "ctrl+h":
		if m.query != "" {
			runes := []rune(m.query)
			m.query = string(runes[:len(runes)-1])
			m.applyFilter()
		}
	case "enter":
		if script, ok := m.selected(); ok {
			m.mode = modeArgs
			m.argInput = ""
			m.errText = ""
			m.result.script = script
		}
	case "ctrl+r":
		if script, ok := m.selected(); ok {
			m.result = finderResult{action: finderRun, script: script}
			return m, tea.Quit
		}
	case "ctrl+d":
		if script, ok := m.selected(); ok {
			m.result = finderResult{action: finderDryRun, script: script}
			return m, tea.Quit
		}
	case "ctrl+o":
		if script, ok := m.selected(); ok {
			m.result = finderResult{action: finderInfo, script: script}
			return m, tea.Quit
		}
	default:
		if len(msg.Runes) > 0 {
			m.query += string(msg.Runes)
			m.applyFilter()
		}
	}
	return m, nil
}

func (m finderModel) handleArgKey(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.errText = ""
	switch key {
	case "ctrl+c":
		m.result.action = finderNone
		return m, tea.Quit
	case "esc":
		m.mode = modeSearch
	case "backspace", "ctrl+h":
		if m.argInput != "" {
			runes := []rune(m.argInput)
			m.argInput = string(runes[:len(runes)-1])
		}
	case "enter":
		args, err := splitArgs(m.argInput)
		if err != nil {
			m.errText = err.Error()
			return m, nil
		}
		m.result.args = args
		m.result.action = finderRun
		return m, tea.Quit
	case "ctrl+d":
		args, err := splitArgs(m.argInput)
		if err != nil {
			m.errText = err.Error()
			return m, nil
		}
		m.result.args = args
		m.result.action = finderDryRun
		return m, tea.Quit
	default:
		if len(msg.Runes) > 0 {
			m.argInput += string(msg.Runes)
		}
	}
	return m, nil
}

func (m finderModel) View() string {
	if m.width < 1 {
		m.width = 100
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Script Librarian"))
	b.WriteString("  ")
	b.WriteString(mutedStyle.Render(m.cfg.ScriptsDir))
	b.WriteString("\n")
	if m.mode == modeArgs {
		b.WriteString(m.renderArgBar())
	} else {
		b.WriteString(m.renderSearchBar())
	}
	b.WriteString("\n\n")
	b.WriteString(m.renderBody())
	b.WriteString("\n")
	if m.errText != "" {
		b.WriteString(errorStyle.Render(m.errText))
		b.WriteString("\n")
	}
	b.WriteString(m.renderHelp())
	return b.String()
}

func (m finderModel) renderSearchBar() string {
	query := m.query
	if query == "" {
		query = mutedStyle.Render("type to search by command, description, tag, alias, usage...")
	}
	count := fmt.Sprintf("%d/%d", len(m.filtered), len(m.scripts))
	return fmt.Sprintf("Search: %s  %s", query, mutedStyle.Render(count))
}

func (m finderModel) renderArgBar() string {
	script, _ := m.selected()
	usage := script.Metadata.Usage
	if usage == "" {
		usage = script.Metadata.Command
	}
	return fmt.Sprintf("Run: %s\nUsage: %s\nArgs: %s", commandStyle.Render(script.Metadata.Command), mutedStyle.Render(usage), m.argInput)
}

func (m finderModel) renderBody() string {
	listWidth := m.width / 2
	if listWidth < 36 {
		listWidth = 36
	}
	if listWidth > 58 {
		listWidth = 58
	}
	detailWidth := m.width - listWidth - 4
	if detailWidth < 32 {
		detailWidth = 32
	}
	list := m.renderList(listWidth)
	detail := m.renderDetail(detailWidth)
	return lipgloss.JoinHorizontal(lipgloss.Top, panelStyle.Width(listWidth).Render(list), selectedPanel.Width(detailWidth).Render(detail))
}

func (m finderModel) renderList(width int) string {
	if len(m.filtered) == 0 {
		return mutedStyle.Render("No matching scripts")
	}
	rows := visibleRows(m.height)
	if rows < 4 {
		rows = 4
	}
	m.ensureCursorVisible()
	end := m.offset + rows
	if end > len(m.filtered) {
		end = len(m.filtered)
	}
	var lines []string
	for i := m.offset; i < end; i++ {
		script := m.filtered[i]
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			prefix = "> "
			style = selectedStyle
		}
		line := fmt.Sprintf("%s%-18s %s", prefix, script.Metadata.Command, script.Metadata.Description)
		line = truncate(line, width-2)
		lines = append(lines, style.Render(line))
	}
	return strings.Join(lines, "\n")
}

func (m finderModel) renderDetail(width int) string {
	script, ok := m.selected()
	if !ok {
		return mutedStyle.Render("Select a script to preview metadata.")
	}
	meta := script.Metadata
	var lines []string
	lines = append(lines, commandStyle.Render(meta.Command))
	if meta.Name != "" {
		lines = append(lines, meta.Name)
	}
	lines = append(lines, "")
	lines = append(lines, wrap(meta.Description, width)...)
	lines = append(lines, "")
	lines = append(lines, field("Usage", meta.Usage))
	lines = append(lines, field("Safety", safetyStyle.Render(meta.Safety)))
	lines = append(lines, field("Runtime", meta.Runtime))
	lines = append(lines, field("Tags", strings.Join(meta.Tags, ", ")))
	if len(meta.Aliases) > 0 {
		lines = append(lines, field("Aliases", strings.Join(meta.Aliases, ", ")))
	}
	if len(meta.Dependencies) > 0 {
		lines = append(lines, field("Deps", strings.Join(meta.Dependencies, ", ")))
	}
	if len(meta.Examples) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Examples:")
		for _, example := range meta.Examples {
			lines = append(lines, "  "+truncate(example, width-4))
		}
	}
	lines = append(lines, "")
	lines = append(lines, mutedStyle.Render(truncate(script.Path, width)))
	return strings.Join(lines, "\n")
}

func (m finderModel) renderHelp() string {
	if m.mode == modeArgs {
		return mutedStyle.Render("enter run   ctrl+d dry-run   esc back   quotes supported for spaced args")
	}
	return mutedStyle.Render("type search   up/down move   enter args   ctrl+r run   ctrl+d dry-run   ctrl+o info   esc quit")
}

func (m *finderModel) applyFilter() {
	m.filtered = searchScripts(m.scripts, m.query)
	if m.query == "" {
		m.filtered = m.scripts
	}
	if len(m.filtered) == 0 {
		m.cursor = 0
		m.offset = 0
		return
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.offset = 0
	m.ensureCursorVisible()
}

func (m *finderModel) move(delta int) {
	if len(m.filtered) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	m.ensureCursorVisible()
}

func (m *finderModel) ensureCursorVisible() {
	rows := visibleRows(m.height)
	if rows < 1 {
		rows = 1
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m finderModel) selected() (Script, bool) {
	if len(m.filtered) == 0 || m.cursor < 0 || m.cursor >= len(m.filtered) {
		return Script{}, false
	}
	return m.filtered[m.cursor], true
}

func visibleRows(height int) int {
	rows := height - 10
	if rows < 6 {
		return 6
	}
	return rows
}

func field(label, value string) string {
	if value == "" {
		value = "-"
	}
	return fmt.Sprintf("%-8s %s", label+":", value)
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width == 1 {
		return "."
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}

func wrap(s string, width int) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{mutedStyle.Render("No description.")}
	}
	var lines []string
	var line string
	for _, word := range strings.Fields(s) {
		if len([]rune(line))+len([]rune(word))+1 > width && line != "" {
			lines = append(lines, line)
			line = word
		} else if line == "" {
			line = word
		} else {
			line += " " + word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
