package terminal

import (
	"context"
	"fmt"

	"github.com/matst80/go-ai-cli/pkg/config"
	"github.com/matst80/go-ai-cli/pkg/ollama"
)

// ToolUI abstracts the user interface interactions for tool execution
type ToolUI interface {
	ConfirmCommand(cmd string) bool
	LogActivity(activity string)
	LogOutput(output string)
}

// ToolExecutor handles the execution of AI tools
type ToolExecutor struct{}

// NewToolExecutor creates a new ToolExecutor
func NewToolExecutor() *ToolExecutor {
	return &ToolExecutor{}
}

// HandleToolCall processes a tool call from the AI
func (e *ToolExecutor) HandleToolCall(ctx context.Context, tc ollama.ToolCall, ui ToolUI) (string, error) {
	switch tc.Function.Name {
	case "execute":
		var args struct {
			Command string `json:"command"`
		}
		if err := ollama.ParseToolArguments(tc.Function.Arguments, &args); err != nil {
			return "", err
		}

		shouldRun := ui.ConfirmCommand(args.Command)

		if !shouldRun {
			return "Cancelled by user", nil
		}

		ui.LogActivity(fmt.Sprintf("**Running:** `%s`", args.Command))
		output, err := RunCommand(args.Command)
		if output != "" {
			ui.LogOutput(output)
		}
		if err != nil {
			return output + "\nError: " + err.Error(), nil
		}
		return output, nil

	case "web_search":
		var args struct {
			Query   string `json:"query"`
			Country string `json:"country"`
			Count   int    `json:"count"`
			Offset  int    `json:"offset"`
		}
		if err := ollama.ParseToolArguments(tc.Function.Arguments, &args); err != nil {
			return "", err
		}

		ui.LogActivity(fmt.Sprintf("**Searching:** `%s`", args.Query))
		output, err := BraveSearch(args.Query, args.Country, args.Count, args.Offset)
		if err != nil {
			output = fmt.Sprintf("Error: %v", err)
		}
		ui.LogOutput(output)
		return output, nil

	case "browser":
		var args struct {
			URL    string `json:"url"`
			Action string `json:"action"`
		}
		if err := ollama.ParseToolArguments(tc.Function.Arguments, &args); err != nil {
			return "", err
		}

		ui.LogActivity(fmt.Sprintf("**Browsing:** `%s` *(%s)*", args.URL, args.Action))
		output, err := ChromeCDP(args.URL, args.Action)
		if err != nil {
			output = fmt.Sprintf("Error: %v", err)
		}
		ui.LogOutput(output)
		return output, nil

	case "remember":
		var args struct {
			Info string `json:"info"`
		}
		if err := ollama.ParseToolArguments(tc.Function.Arguments, &args); err != nil {
			return "", err
		}

		cfg, _ := config.Load()
		if cfg == nil {
			cfg = &config.Config{}
		}
		cfg.Memory = append(cfg.Memory, args.Info)
		_ = cfg.Save()

		ui.LogActivity(fmt.Sprintf("**Remembered:** %s", args.Info))
		return "Memory saved", nil

	case "set_system_prompt":
		var args struct {
			Prompt string `json:"prompt"`
		}
		if err := ollama.ParseToolArguments(tc.Function.Arguments, &args); err != nil {
			return "", err
		}

		cfg, _ := config.Load()
		if cfg == nil {
			cfg = &config.Config{}
		}
		cfg.SystemPrompt = args.Prompt
		_ = cfg.Save()

		ui.LogActivity("**System prompt updated**")
		return "System prompt updated", nil

	default:
		return "", fmt.Errorf("unknown tool: %s", tc.Function.Name)
	}
}
