# User Experience

## Experience Principles

- The CLI should feel immediate.
- The user should always know where scripts live.
- The app should prefer reuse over duplication.
- Generated scripts should be reviewable before they become trusted tools.
- Safety should be visible, not buried.
- The product should help agents behave well without requiring a full agent
  integration.

## First Run

Command:

```bash
msl init
```

Flow:

```text
Welcome to Script Librarian.

Where should managed scripts live?
> ~/scripts

Initialize git history there?
> yes

Create command shims in ~/.local/bin?
> yes

Configure an LLM provider now?
> yes

Install MCP config / print agent instructions?
> print instructions
```

Output:

```text
Script library ready:
  Scripts: ~/scripts
  Git: enabled
  Shims: ~/.local/bin
  Config: ~/.config/script-librarian/config.toml

Run `msl new "..."` to create a script.
Run `msl` to search your library.
```

## Finding Scripts

Command:

```bash
msl
```

Interactive picker:

```text
Search scripts
> repo clean

  repo-cleanup       Remove merged git branches after confirmation.
  repo-status        Summarize branches, remotes, and local changes.
  env-unused         Find possibly unused environment variables.

Safety: writes-project
Usage:  repo-cleanup [--dry-run]
Path:   ~/scripts/git/repo-cleanup
```

Non-interactive:

```bash
msl search "repo clean"
msl info repo-cleanup
msl run repo-cleanup -- --dry-run
```

## Generating Scripts

Command:

```bash
msl new "make a script that finds large files in this repo"
```

Flow:

```text
Searching existing scripts...

Similar scripts:
  large-files        Find large files below the current directory.
  repo-disk-usage    Summarize disk usage by folder.

Create a new script or improve an existing one?
> Create new
```

Generation:

```text
Provider: ollama / gemma
Runtime: bash
Safety: read-only

Generated:
  Name: Large Files
  Command: large-files
  Usage: large-files [--min-size 100M] [path]
```

Review:

```text
Review generated script

+ #!/usr/bin/env bash
+ # msl:name Large Files
+ # msl:description Find large files below a path.
+ # msl:usage large-files [--min-size 100M] [path]
+ # msl:tags files, disk, cleanup
+ # msl:safety read-only

Save this script?
> yes
```

Save:

```text
Saved: ~/scripts/files/large-files
Made executable.
Committed: Add large-files script
```

## Improving Scripts

Command:

```bash
msl improve repo-cleanup "add --dry-run and better branch filtering"
```

Flow:

```text
Loaded repo-cleanup.
Generating update...
Validation passed:
  metadata present
  help works
  dry-run works

Review diff?
> yes
```

## Version Control

Commands:

```bash
msl scripts status
msl scripts log repo-cleanup
msl scripts diff repo-cleanup
msl scripts rollback repo-cleanup
msl scripts sync
```

Expected behavior:

- Script changes are visible as git diffs.
- Agent-generated changes include the original prompt in the commit message or
  metadata.
- Rollback restores previous versions through git.

## LLM Configuration

Command:

```bash
msl llm configure
```

Flow:

```text
Choose provider:
  OpenAI-compatible
  Ollama
  LM Studio
  Anthropic
  OpenAI
  custom command

Base URL:
> http://localhost:11434/v1

Model:
> gemma

Test provider?
> yes
```

## MCP Agent Flow

External agent behavior:

```text
1. Ask Script Librarian for script shape guidance.
2. Search existing scripts before creating a new one.
3. Prefer updating an existing script when appropriate.
4. Save scripts through MCP instead of arbitrary filesystem writes.
5. Let Script Librarian validate and version the change.
```

Possible MCP tools:

```text
list_scripts
search_scripts
get_script
get_script_shape
validate_script
save_script
update_script
```

Execution tools should be conservative:

```text
run_script_dry
run_script_requires_user_confirmation
```

## Agent Instructions

The app should be able to print or install a reusable instruction block:

```text
Before creating reusable local utility scripts, query Script Librarian through
MCP. If a suitable script already exists, use or update it. If creating a new
script, save it through Script Librarian and follow the required metadata shape.
```

## Failure States

Provider unavailable:

```text
Could not reach the configured LLM endpoint.
Run `msl llm configure` or retry with `--provider`.
```

Unsafe generation:

```text
This script appears destructive but does not declare a destructive safety level
or confirmation behavior. Save blocked until reviewed.
```

Duplicate script:

```text
This looks similar to repo-cleanup.
Create a new script anyway, or improve repo-cleanup?
```
