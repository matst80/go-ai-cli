# Go AI CLI

A simple CLI tool to interact with Ollama, specifically designed to help with terminal commands.

## Features

- **Streaming Responses**: Get immediate feedback as the AI generates text.
- **Markdown Formatting**: Renders markdown (code blocks, bold, etc.) beautifully in your terminal using [Glamour](https://github.com/charmbracelet/glamour).
- **Auto-suggest Commands**: On Unix systems (Linux, macOS), the AI can suggest a command that will be automatically typed onto your terminal's next line after the tool exits.
- **Terminal Helper**: Pre-configured with a system message to help with shell commands.
- **Web Search**: Integrates with Brave Search API to find information on the web.
- **Browser (CDP)**: Controls Chrome via CDP to scrape sites, take screenshots, or navigate to pages in a persistent browser window.

## Installation

### From Source (Local Build)

To build the tool locally:

```bash
go build -o ai main.go
```

### Global Installation

To install the tool globally so it's available as `go-ai-cli` from anywhere:

```bash
go install github.com/matst80/go-ai-cli@latest
```

Ensure your `GOPATH/bin` is in your `PATH`.

Alternatively, if you've already built the binary as `ai` and want to move it to a global location (e.g., `/usr/local/bin`):

```bash
sudo mv ai /usr/local/bin/
```

## Configuration

The tool can be configured using the following environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `OLLAMA_URL` | The URL for the Ollama chat API. | `http://localhost:11434/api/chat` |
| `OLLAMA_MODEL` | The model name to use. | `ministral-3:latest` |
| `BRAVE_API_KEY` | API key for Brave Search. Required to enable the `web_search` tool. | (None) |
| `CHROME_REMOTE_URL` | URL or port for an existing Chrome instance to control via CDP (e.g., `9222` or `localhost:9222`). Can also be set via the `--cdp` flag. | (None, starts managed instance) |
| `AI_STYLE` | Set the output theme (e.g., `dark`, `light`, `auto`). | `auto` |
| `AI_YOLO` | Run all commands without confirmation. | `false` |
| `GLAMOUR_STYLE` | Fallback environment variable for the output theme. Compatible with `glow`. | (None) |

Example:
```bash
export OLLAMA_URL="http://10.10.10.108:11434/api/chat"
export BRAVE_API_KEY="your_api_key_here"
export AI_YOLO=true
```

## Usage

```bash
./ai "How do I list files by size in the current directory?"
```

### Confirmation and YOLO

By default, any `run_command` tool execution will prompt for confirmation. You can bypass this using the `--yolo` flag or `AI_YOLO=true` environment variable.

```bash
# Ask for confirmation (default)
./ai "run tests for this project"

# Run everything immediately
./ai --yolo "run tests for this project"
```

### Styling

The tool automatically detects your terminal's background color. If you need to manually force a theme:

```bash
# Force light mode via flag
./ai --style light "how to use grep?"

# Force dark mode via environment variable
export AI_STYLE=dark
./ai "how to use grep?"
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

### Multimodal and File Support

The CLI supports multimodal models and can process local files.

#### Pipe an image or file:
```bash
cat image.png | ./ai "describe this image"
cat main.go | ./ai "suggest improvements"
```

#### Append files as arguments:
```bash
./ai "explain this code" pkg/ollama/client.go
./ai "compare these images" car1.jpg car2.jpg
```

The tool automatically detects image files and sends them to Ollama. Other files are appended to the prompt as text.

## Testing

```bash
go test ./...
```
