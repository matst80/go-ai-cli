# Go AI CLI

A simple CLI tool to interact with Ollama, specifically designed to help with terminal commands.

## Features

- **Streaming Responses**: Get immediate feedback as the AI generates text.
- **Markdown Formatting**: Renders markdown (code blocks, bold, etc.) beautifully in your terminal using [Glamour](https://github.com/charmbracelet/glamour).
- **Auto-suggest Commands**: On macOS, the AI can suggest a command that will be automatically typed onto your terminal's next line after the tool exits.
- **Terminal Helper**: Pre-configured with a system message to help with shell commands.
- **Web Search**: Integrates with Brave Search API to find information on the web.
- **Browser (CDP)**: Controls Chrome via CDP to scrape sites, take screenshots, or navigate to pages in a persistent browser window.

## Installation

```bash
go build -o ai main.go
```

## Usage

```bash
./ai "How do I list files by size in the current directory?"
```

### Persistent Browser (CDP)

The CLI can control an existing Chrome instance for debugging or multi-step browsing. To do this, start Chrome with the remote debugging port enabled:

```bash
# macOS
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --remote-debugging-port=9222 --user-data-dir="$HOME/.go-ai-cli/chrome-profile"
```

Then use the `--cdp` flag:

```bash
# Connect using port
./ai --cdp 9222 "scrape google.com"

# Connect using full URL
./ai --cdp localhost:9222 "screenshot https://github.com"
```

If not specified, the tool will attempt to start its own managed Chrome instance.

## Testing

```bash
go test ./...
```
