package app

import (
	"fmt"
	"io"
	"strings"
)

func printScriptList(w io.Writer, scripts []Script) {
	for _, script := range scripts {
		fmt.Fprintf(w, "%-18s %s\n", script.Metadata.Command, script.Metadata.Description)
	}
}

func printScriptInfo(w io.Writer, script Script) {
	m := script.Metadata
	fmt.Fprintf(w, "Command: %s\n", m.Command)
	fmt.Fprintf(w, "Name: %s\n", m.Name)
	fmt.Fprintf(w, "Description: %s\n", m.Description)
	fmt.Fprintf(w, "Usage: %s\n", m.Usage)
	fmt.Fprintf(w, "Tags: %s\n", strings.Join(m.Tags, ", "))
	fmt.Fprintf(w, "Aliases: %s\n", strings.Join(m.Aliases, ", "))
	fmt.Fprintf(w, "Runtime: %s\n", m.Runtime)
	fmt.Fprintf(w, "Safety: %s\n", m.Safety)
	fmt.Fprintf(w, "Dependencies: %s\n", strings.Join(m.Dependencies, ", "))
	if len(m.Examples) > 0 {
		fmt.Fprintf(w, "Examples:\n")
		for _, example := range m.Examples {
			fmt.Fprintf(w, "  %s\n", example)
		}
	}
	fmt.Fprintf(w, "Path: %s\n", script.Path)
}

func scriptShapeGuidance() string {
	return `Every managed script should include top-of-file metadata comments:

#!/usr/bin/env bash
# msl:name Display Name
# msl:description One sentence describing the script.
# msl:usage command-name [options] [args]
# msl:tags tag-one, tag-two
# msl:runtime bash
# msl:safety read-only
# msl:deps git, jq
# msl:example command-name --help

Valid safety levels: read-only, writes-project, writes-home, network, destructive, requires-confirmation.`
}
