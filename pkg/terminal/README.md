# Terminal Package

This package manages the Command Line Interface (CLI) functionality, including TTY/Bubble Tea integration and terminal-specific helpers.

## Key Components

- `UI`: Bubble Tea model (`tea.Model`) for interactive chat.
- `RunInteractiveSession`: Handles the logic loop for a TTY, including tool orchestration.
- `RunSimpleSession`: Fallback for non-TTY environments.
- `RunCommand`: Safe execution of shell commands.
- `ExtractCommandFromMarkdown`: Utility to find suggested commands in markdown.
- `HandleSuggestedCommand`: macOS-specific helper using `osascript` to suggest commands at the prompt.
