# Product Requirements

## Summary

Script Librarian is a local CLI for managing reusable scripts. It helps users
discover existing scripts, generate new scripts through a configured LLM
endpoint, save scripts into a managed library, version changes with git, and
expose the library to external AI agents through MCP.

## Problem

Developers and power users repeatedly create small utility scripts, but those
scripts often end up scattered across projects, shell history, dotfiles, chat
threads, and one-off agent outputs. Even when scripts exist, future humans and
agents often do not know they exist, do not understand their safety profile, or
cannot confidently update them.

## Product Goal

Create a trusted local script library that both humans and AI assistants can use.

The product should make it natural to:

- Find the right script.
- Understand what it does before running it.
- Generate a new script when none exists.
- Update an existing script instead of duplicating it.
- Save generated scripts into a managed location.
- Keep a useful git history of script changes.
- Let external agents discover and contribute scripts safely.

## Non-Goals

- Do not become a full coding agent.
- Do not require a specific LLM provider.
- Do not require a specific implementation language or runtime yet.
- Do not replace the user's shell.
- Do not execute unsafe scripts without clear user intent.
- Do not hide generated changes from the user.

## Personas

### Local Power User

Wants quick access to reusable shell utilities without remembering filenames,
paths, or exact arguments.

### AI-Assisted Developer

Uses coding agents or local LLMs and wants useful generated scripts to persist
after the current chat/session ends.

### Future Agent

Needs to know what scripts already exist, what shape new scripts should follow,
and where managed scripts should be saved.

## Core Concepts

### Managed Script Home

During setup, the user chooses where managed scripts live. This may be a
directory such as `~/scripts`, `~/.local/share/script-librarian/scripts`, or a
dedicated git repository.

### Script Shape

Every managed script should include structured metadata. The metadata must be
simple enough for humans to write and reliable enough for LLMs to follow.

Required fields:

- name
- description
- usage
- tags
- runtime
- safety level
- dependencies
- examples

Recommended fields:

- aliases
- inputs
- outputs
- failure modes
- test command
- created by
- updated by

### Safety Level

Scripts should declare a safety level. Early levels:

- `read-only`
- `writes-project`
- `writes-home`
- `network`
- `destructive`
- `requires-confirmation`

### Versioned Library

The managed script home should be a git repository or live inside one. Generated
or updated scripts should produce reviewable diffs and meaningful commits.

### LLM Provider Adapter

The built-in generator should call a configured LLM endpoint through a small
adapter layer. The product should not assume a heavyweight coding agent.

Initial adapter targets:

- OpenAI-compatible endpoint
- Ollama
- LM Studio
- Anthropic
- OpenAI
- custom command

### MCP Surface

MCP should allow external agents to query, validate, and save scripts through
the same managed library. MCP is an integration surface, not the primary
generation path.

## Key User Stories

As a user, I can initialize the library and choose where scripts are saved.

As a user, I can search all managed scripts by command name, description, tags,
aliases, usage, and examples.

As a user, I can preview a script's command, metadata, source path, and safety
level before executing it.

As a user, I can generate a script from a prompt using my configured LLM
endpoint.

As a user, I can see similar existing scripts before creating a new script.

As a user, I can review generated script content before saving it.

As a user, I can validate a generated script before trusting it.

As a user, I can version script changes with git.

As an external AI agent, I can query existing scripts before creating a new one.

As an external AI agent, I can save or update a script through a controlled MCP
tool instead of writing to an arbitrary path.

## MVP Scope

### Setup

- Choose managed script directory.
- Optionally initialize git.
- Optionally create shell shims or symlinks.
- Write local config.
- Print agent instructions.

### Discovery

- Scan managed scripts.
- Parse metadata.
- Search scripts.
- Show script details.

### Execution

- Run a selected script.
- Support dry-run display.
- Require confirmation for scripts marked destructive or requiring
  confirmation.

### Generation

- Configure one LLM provider.
- Search for similar scripts before generation.
- Generate one script plus metadata.
- Validate basic script shape.
- Show diff/review before save.
- Save, chmod, and optionally commit.

### MCP

- List scripts.
- Search scripts.
- Get script details.
- Get script shape guidance.
- Validate script text.
- Save script with explicit metadata.

## Later Scope

- Script improvement workflows.
- Script rollback.
- Dependency detection and install guidance.
- Script test harnesses.
- Team/shared script libraries.
- Signed or trusted scripts.
- Rich sidecar metadata.
- Usage analytics stored locally.
- Multi-model generation and review.

## Open Questions

- What is the default managed script directory?
- Should the default storage be a plain directory or a git repo?
- Should metadata live only in comments, or comments plus sidecar files?
- How much validation should happen before save?
- Should execution through MCP be allowed at all in v1?
- What is the right default command name: `msl`, `sl`, or something else?
- What installation path should be optimized first?
- Which implementation language/toolchain best matches the performance goals?
