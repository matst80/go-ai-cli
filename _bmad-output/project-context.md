---
project_name: 'go-ai-cli'
user_name: 'Mats'
date: '2026-03-05T20:06:06Z'
sections_completed: ['technology_stack']
existing_patterns_found: 0
---

# Project Context for AI Agents

_This file contains critical rules and patterns that AI agents must follow when implementing code in this project. Focus on unobvious details that agents might otherwise miss._

---

## Technology Stack & Versions

### Go Environment
- **Go Version**: 1.25.1 (from go.mod)

### Key Go Libraries
- `github.com/charmbracelet/bubbles` v1.0.0 — component library for terminal UI
- `github.com/charmbracelet/bubbletea` v1.3.10 — framework for building terminal UIs
- `github.com/charmbracelet/glamour` v0.10.0 — markdown rendering in terminal
- `github.com/charmbracelet/lipgloss` v1.1.1-0.20250404203927-76690c660834 — styling and layout for terminal
- `github.com/chromedp/chromedp` v0.14.2 — Chrome/Chromium automation via CDP
- `github.com/mattn/go-isatty` v0.0.20 — detect if output is interactive terminal
- `golang.design/x/clipboard` v0.7.1 — clipboard access

### Node Tooling
- `@opencode-ai/plugin` v1.2.15 (from .opencode/package.json)

### Project Architecture Notes
- **Language**: Go (primary)
- **UI Framework**: TUI (Terminal User Interface) using Charmbracelet libraries
- **Browser Automation**: Chrome/Chromium via ChromeDP
- **Entry Point**: main.go
- **Package Structure**: pkg/{area} layout with main.go as entry point
- **CLI Features**: Streaming responses, markdown rendering, web search, browser control, file creation

---

## Critical Implementation Rules

1. **Maintain Go module compatibility** — Use Go 1.25.1 from go.mod. Do not introduce dependencies outside the go.mod specification without explicit approval.

2. **Follow existing package layout** — Code belongs in `pkg/{area}` subdirectories with `main.go` as the entry point. Preserve this structure for new features.

3. **Preserve TUI rendering patterns** — Use Charmbracelet libraries (bubbles, bubbletea, glamour, lipgloss) for terminal UI. Follow existing component patterns and do not introduce conflicting UI frameworks.

4. **Keep Go testing conventions** — Use `*_test.go` files and follow existing test scaffolding. Maintain test coverage for new functionality.

5. **For UI/terminal code changes, ensure behavior is preserved and add tests** — Any modifications to TUI rendering, markdown handling, or terminal output must maintain backward compatibility and include test verification.

6. **Do not fabricate undocumented architecture details** — Require explicit documentation or user confirmation before assuming project design decisions not evident in code or README.

---

_Last generated: 2026-03-05T20:06:06Z_
