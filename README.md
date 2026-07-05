# Script Librarian

Script Librarian is a native CLI for finding, generating, managing, versioning,
and running reusable local scripts. The command name is `msl`.

## Product Promise

Your local reusable automation library, shared between you and your AI assistants.

## Core Ideas

- Keep useful scripts in one managed place.
- Make scripts easy to find, inspect, and run.
- Give scripts a predictable metadata shape so humans and LLMs understand them.
- Generate simple scripts through a configurable LLM endpoint.
- Let external agents query and save scripts through MCP.
- Version all script changes with git.

## Current Status

The MVP is implemented as a small Go CLI with no third-party runtime
dependencies after build. The interactive finder uses Bubble Tea and Lip Gloss.
It supports local library setup, metadata scanning, TUI search with metadata
preview, script details, guarded execution, script generation through provider
adapters, git-backed script history commands, and a stdio MCP surface for
external agents.

## Local Development

Requirements:

- Go 1.26 or newer
- Git
- Bash for the end-to-end smoke test

Common commands:

```bash
make test
make build
make e2e
```

The compiled binary is written to `bin/msl`.

## CLI Quick Start

```bash
msl init
msl
msl search "repo clean"
msl info repo-cleanup
msl run repo-cleanup -- --dry-run
msl repo-cleanup --dry-run
msl new "make a script that finds large files in this repo"
msl scripts status
msl mcp
msl mcp instructions
```

For non-interactive setup:

```bash
msl init --dir ~/scripts --shims ~/.local/bin --yes
```

Local configuration is stored at:

```text
~/.config/script-librarian/config.toml
```

## Interactive Finder

Run `msl` with no arguments to open the TUI finder.

The finder shows matched scripts on the left and metadata for the selected
script on the right, including description, usage, safety, runtime, tags,
aliases, dependencies, examples, and path.

Shortcuts:

- Type to filter by command, description, tags, aliases, usage, and examples.
- Use `up` / `down` to move through matches.
- Press `enter` to open the argument prompt for the selected script.
- Press `ctrl+r` to run the selected script with no extra args.
- Press `ctrl+d` to dry-run the selected script or argument prompt.
- Press `ctrl+o` to print full script metadata and exit.
- Press `esc` or `ctrl+c` to quit.

The argument prompt supports shell-style spacing with single quotes, double
quotes, and backslash escapes:

```text
--name "Ada Lovelace" --path 'folder with spaces'
```

If you already know the command or alias, use the fast path:

```bash
msl repo-cleanup --dry-run
msl rc --dry-run
```

## Script Metadata

Managed scripts use top-of-file comments:

```bash
#!/usr/bin/env bash
# msl:name Repo Cleanup
# msl:description Remove merged local git branches after confirmation.
# msl:usage repo-cleanup [--dry-run]
# msl:tags git, cleanup, branches
# msl:runtime bash
# msl:safety writes-project
# msl:deps git
# msl:example repo-cleanup --dry-run
```

Valid safety levels are `read-only`, `writes-project`, `writes-home`,
`network`, `destructive`, and `requires-confirmation`.

## LLM Providers

Configure the default OpenAI-compatible endpoint:

```bash
msl llm configure \
  --provider openai-compatible \
  --base-url http://localhost:11434/v1 \
  --model gemma
```

For local testing or custom generators:

```bash
msl llm configure --provider custom --custom-command ./generate-script.sh
```

The custom command receives the generation prompt on stdin and must print one
complete script on stdout.

## MCP

Run the stdio MCP server:

```bash
msl mcp
```

Available tools:

- `list_scripts`
- `search_scripts`
- `get_script`
- `get_script_shape`
- `validate_script`
- `save_script`
- `update_script`

Execution through MCP is intentionally omitted from the MVP.

Print reusable instructions for agents:

```bash
msl mcp instructions
```

## CI/CD

CI runs formatting checks, unit tests, build, and the end-to-end smoke test on
pushes to `main` and pull requests.

Tagged releases matching `v*` build native binaries for Linux, macOS, and
Windows, then publish them to GitHub Releases with SHA-256 checksums.

## Docs

- [Product Requirements](docs/PRD.md)
- [User Experience](docs/UX.md)
- [Design Direction](docs/DESIGN.md)

## Previous Project

This direction replaces the archived
[min-script-launcher](https://github.com/cmwen/min-script-launcher) project.
