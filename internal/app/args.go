package app

import (
	"fmt"
	"strings"
	"unicode"
)

func splitArgs(input string) ([]string, error) {
	var args []string
	var b strings.Builder
	var quote rune
	escaped := false
	inArg := false
	for _, r := range input {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
			inArg = true
		case r == '\\':
			escaped = true
			inArg = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
			inArg = true
		case r == '\'' || r == '"':
			quote = r
			inArg = true
		case unicode.IsSpace(r):
			if inArg {
				args = append(args, b.String())
				b.Reset()
				inArg = false
			}
		default:
			b.WriteRune(r)
			inArg = true
		}
	}
	if escaped {
		return nil, fmt.Errorf("unfinished escape sequence")
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	if inArg {
		args = append(args, b.String())
	}
	return args, nil
}

func quoteArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "" {
			out = append(out, "''")
			continue
		}
		needsQuote := false
		for _, r := range arg {
			if unicode.IsSpace(r) || r == '\'' || r == '"' || r == '\\' {
				needsQuote = true
				break
			}
		}
		if !needsQuote {
			out = append(out, arg)
			continue
		}
		out = append(out, "'"+strings.ReplaceAll(arg, "'", `'\''`)+"'")
	}
	return out
}
