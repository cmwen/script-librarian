package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func scanScripts(dir string) ([]Script, error) {
	var scripts []Script
	if dir == "" {
		return nil, errors.New("scripts directory is not configured")
	}
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		meta, err := parseScriptFile(path)
		if err != nil {
			return err
		}
		if len(meta.Raw) == 0 {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		scripts = append(scripts, Script{Path: path, RelPath: rel, Metadata: meta, ModTime: info.ModTime(), Size: info.Size()})
		return nil
	})
	sort.Slice(scripts, func(i, j int) bool {
		return scripts[i].Metadata.Command < scripts[j].Metadata.Command
	})
	return scripts, err
}

func findScript(dir, name string) (Script, error) {
	scripts, err := scanScripts(dir)
	if err != nil {
		return Script{}, err
	}
	for _, script := range scripts {
		if script.Metadata.Command == name || script.Metadata.Name == name || contains(script.Metadata.Aliases, name) {
			return script, nil
		}
	}
	return Script{}, fmt.Errorf("script not found: %s", name)
}

func searchScripts(scripts []Script, query string) []Script {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return scripts
	}
	type scored struct {
		script Script
		score  int
	}
	words := strings.Fields(query)
	var hits []scored
	for _, script := range scripts {
		hay := strings.ToLower(strings.Join([]string{
			script.Metadata.Command,
			script.Metadata.Name,
			script.Metadata.Description,
			script.Metadata.Usage,
			strings.Join(script.Metadata.Tags, " "),
			strings.Join(script.Metadata.Aliases, " "),
			strings.Join(script.Metadata.Examples, " "),
		}, " "))
		score := 0
		for _, word := range words {
			if script.Metadata.Command == word {
				score += 20
			}
			if strings.Contains(strings.ToLower(script.Metadata.Command), word) {
				score += 10
			}
			if strings.Contains(hay, word) {
				score += 3
			}
		}
		if score > 0 {
			hits = append(hits, scored{script: script, score: score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score == hits[j].score {
			return hits[i].script.Metadata.Command < hits[j].script.Metadata.Command
		}
		return hits[i].score > hits[j].score
	})
	out := make([]Script, 0, len(hits))
	for _, hit := range hits {
		out = append(out, hit.script)
	}
	return out
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
