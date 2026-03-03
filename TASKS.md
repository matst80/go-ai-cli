# Task List: Features and Refactoring

## Features to Add (Local AI Helper Focus)

1. **Local File System Tools**
   - Add a `list_files` tool so the AI can explore directories without writing Bash commands (safer and more reliable).
   - Add a `read_file` tool so the AI can pull in specific files dynamically during a conversation, rather than relying on the user to pipe them in upfront.
   - *Rationale:* Essential for a coding assistant to understand the project structure and contents autonomously.

2. **Conversation History / Session Persistence**
   - Currently, memory is global (`remember`), but the *chat history* itself resets every run.
   - Add an option to save/load conversation history to a local SQLite database or JSON file (e.g., `~/.ai-cli/sessions/`).
   - Add a `--resume` flag to continue the last conversation or a specific session ID.

3. **Interactive "Agentic" Loop improvements**
   - The AI can already run commands and see the output, but it could be improved by giving the AI explicit feedback when a command fails or hangs.
   - Add a way for the AI to interrupt a long-running command or run background tasks.

4. **Better Context Management (Token Limits)**
   - When using local LLMs, context windows fill up fast (especially with `browser` scrape or long `execute` outputs).
   - Implement an automatic context truncator or summarizer that kicks in when the message history exceeds a certain token limit, keeping the system prompt and recent messages intact.

5. **Clipboard Integration Tool**
   - Add a `read_clipboard` tool so the AI can access what the user just copied (e.g., an error message from another window).

6. **Syntax Highlighting for Piped Input**
   - When a user pipes a file (`cat main.go | ai "explain"`), the app appends it as text. It would be better to wrap piped code in markdown code blocks before sending it to the LLM to improve the model's understanding.

## Refactoring Tasks

1. **Decouple Tool Implementations from UI Loops**
   - *Current State:* The logic for handling tool calls (`execute`, `web_search`, `browser`, `remember`, `set_system_prompt`) is duplicated in `pkg/terminal/ui.go` and `pkg/terminal/simple.go`.
   - *Refactoring:* Create a central `ToolExecutor` struct/interface in `pkg/tools` or `pkg/terminal`. Both UI runners should call something like `executor.HandleToolCall(tc)` which returns the output string.

2. **Clean up `main.go`**
   - `main.go` currently defines all the `ollama.Tool` JSON schemas inline.
   - *Refactoring:* Move tool definitions to their respective handler packages (e.g., a `GetAvailableTools()` function) to keep `main.go` focused on initialization and dependency injection.

3. **Improve Error Handling in Streaming Loop**
   - In `StreamWorker` and the UI loops, errors sometimes just print and `break`, which can leave the UI in an ambiguous state.
   - *Refactoring:* Standardize error wrapping and ensure errors gracefully stop the spinner and present a clear red error message to the user before exiting.

4. **Configuration Defaults Centralization**
   - *Current State:* `config.Load()` has some hardcoded defaults, but `ui.go` and `viewer.go` also check `os.Getenv("AI_STYLE")` and `os.Getenv("GLAMOUR_STYLE")` independently.
   - *Refactoring:* All environment variable reading should happen *only* in `config.Load()`. Pass the parsed `Config` struct down to the UI components instead of them reading `os.Getenv`.

5. **StreamHandler State Machine**
   - The `StreamHandler` in `filewriter.go` does a lot of string matching for `:::lang` and ````lang`.
   - *Refactoring:* Refactor into a more robust state machine to handle edge cases (like incomplete markdown blocks, or markdown blocks inside other markdown blocks) more reliably.

6. **Testing Coverage**
   - There are test files (`config_test.go`, `browser_test.go`, `client_test.go`), but the core UI loop and tool execution are hard to test because they are tightly coupled.
   - *Refactoring:* By decoupling the tool executor (Refactor #1), it becomes possible to write unit tests for the AI's tool interaction loop using mock LLM responses.
