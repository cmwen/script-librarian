# Script Librarian

Script Librarian is an early product direction for a local CLI that helps people find,
generate, manage, version, and run reusable scripts.

The working command name may still be `msl`, but the product is no longer only a
script launcher. The goal is to become a local automation library shared by a
human user and their AI tools.

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

This repository is intentionally docs-first. The implementation language,
runtime, package format, and TUI framework are not decided yet.

## Docs

- [Product Requirements](docs/PRD.md)
- [User Experience](docs/UX.md)
- [Design Direction](docs/DESIGN.md)

## Previous Project

This direction replaces the archived
[min-script-launcher](https://github.com/cmwen/min-script-launcher) project.
