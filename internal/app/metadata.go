package app

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var commandRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

func parseScriptFile(path string) (Metadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return Metadata{}, err
	}
	defer f.Close()
	return parseMetadata(f, filepath.Base(path))
}

func parseMetadata(r io.Reader, fallbackCommand string) (Metadata, error) {
	meta := Metadata{Command: fallbackCommand, Raw: map[string][]string{}}
	scanner := bufio.NewScanner(r)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		if lineNo > 120 {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" && lineNo > 2 {
			break
		}
		if !strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "#!") && lineNo == 1 {
				continue
			}
			if lineNo > 1 {
				break
			}
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(line, "#"))
		if !strings.HasPrefix(body, "msl:") {
			continue
		}
		key, value := splitMetadataLine(strings.TrimPrefix(body, "msl:"))
		if key == "" {
			continue
		}
		key = normalizeKey(key)
		value = strings.TrimSpace(value)
		meta.Raw[key] = append(meta.Raw[key], value)
		applyMetadata(&meta, key, value)
	}
	if err := scanner.Err(); err != nil {
		return meta, err
	}
	if meta.Command == "" {
		meta.Command = fallbackCommand
	}
	return meta, nil
}

func splitMetadataLine(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	for i, r := range s {
		if r == ':' || r == ' ' || r == '\t' {
			return s[:i], strings.TrimSpace(strings.TrimLeft(s[i:], ": \t"))
		}
	}
	return s, ""
}

func normalizeKey(k string) string {
	k = strings.ToLower(strings.TrimSpace(k))
	k = strings.ReplaceAll(k, "-", "_")
	switch k {
	case "dep", "deps", "dependency":
		return "dependencies"
	case "alias":
		return "aliases"
	case "example":
		return "examples"
	case "safety_level":
		return "safety"
	case "cmd", "command_name":
		return "command"
	default:
		return k
	}
}

func applyMetadata(meta *Metadata, key, value string) {
	switch key {
	case "command":
		meta.Command = slug(value)
	case "name":
		meta.Name = value
	case "description":
		meta.Description = value
	case "usage":
		meta.Usage = value
		if meta.Command == "" || meta.Command == "." {
			meta.Command = commandFromUsage(value)
		}
	case "tags":
		meta.Tags = splitList(value)
	case "aliases":
		meta.Aliases = splitList(value)
	case "runtime":
		meta.Runtime = value
	case "safety":
		meta.Safety = value
	case "dependencies":
		meta.Dependencies = splitList(value)
	case "examples":
		meta.Examples = append(meta.Examples, value)
	case "created_by":
		meta.CreatedBy = value
	case "updated_by":
		meta.UpdatedBy = value
	}
	if key == "usage" {
		if cmd := commandFromUsage(value); cmd != "" {
			meta.Command = cmd
		}
	}
}

func splitList(value string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';'
	}) {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func commandFromUsage(usage string) string {
	usage = strings.TrimSpace(usage)
	if usage == "" {
		return ""
	}
	first := strings.Fields(usage)[0]
	first = filepath.Base(first)
	if commandRe.MatchString(first) {
		return first
	}
	return ""
}

func validateMetadata(meta Metadata, strict bool) ValidationResult {
	var errs []string
	var warnings []string
	required := map[string]string{
		"name":         meta.Name,
		"description":  meta.Description,
		"usage":        meta.Usage,
		"runtime":      meta.Runtime,
		"safety":       meta.Safety,
		"dependencies": strings.Join(meta.Dependencies, ","),
	}
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			errs = append(errs, fmt.Sprintf("missing metadata field: %s", field))
		}
	}
	if strict && len(meta.Examples) == 0 {
		errs = append(errs, "missing metadata field: examples")
	}
	if meta.Command == "" || !commandRe.MatchString(meta.Command) {
		errs = append(errs, "metadata usage must start with a valid command name")
	}
	if len(meta.Tags) == 0 {
		errs = append(errs, "missing metadata field: tags")
	}
	if meta.Safety != "" && !validSafety(meta.Safety) {
		errs = append(errs, "invalid safety level: "+meta.Safety)
	}
	if !strict && len(meta.Examples) == 0 {
		warnings = append(warnings, "missing recommended metadata field: examples")
	}
	return ValidationResult{OK: len(errs) == 0, Errors: errs, Warnings: warnings}
}

func validSafety(s string) bool {
	switch s {
	case "read-only", "writes-project", "writes-home", "network", "destructive", "requires-confirmation":
		return true
	default:
		return false
	}
}

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == '.' || r == ' ':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
