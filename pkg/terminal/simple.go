package terminal

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/matst80/go-ai-cli/pkg/config"
	"github.com/matst80/go-ai-cli/pkg/ollama"
)

// RunSimpleSession provides a non-interactive output for non-TTY or fallback
func RunSimpleSession(client *ollama.Client, req ollama.ChatRequest) (string, error) {
	var preparedCmd string
	for {
		workerCh := make(chan ollama.StreamResponse)
		go client.StreamWorker(context.Background(), req, workerCh)

		var assistantMsg ollama.Message
		assistantMsg.Role = "assistant"

		for msg := range workerCh {
			if msg.Error != nil {
				return "", msg.Error
			}
			if msg.ReasoningContent != "" {
				if assistantMsg.ReasoningContent == "" {
					fmt.Println("_Thinking..._")
				}
				assistantMsg.ReasoningContent += msg.ReasoningContent
				fmt.Print(msg.ReasoningContent)
			}
			if msg.Content != "" {
				if assistantMsg.Content == "" && assistantMsg.ReasoningContent != "" {
					fmt.Print("\n\n---\n\n")
				}
				assistantMsg.Content += msg.Content
				fmt.Print(msg.Content)
			}
			if len(msg.ToolCalls) > 0 {
				assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, msg.ToolCalls...)
				for _, tc := range msg.ToolCalls {
					if tc.Function.Name == "add_command" {
						var args struct {
							Command string `json:"command"`
						}
						if err := ollama.ParseToolArguments(tc.Function.Arguments, &args); err == nil {
							preparedCmd = args.Command
						}
					}
				}
			}
		}

		// Add assistant message to history
		req.Messages = append(req.Messages, assistantMsg)

		// Check for tool calls that need immediate execution
		var toolResponses []ollama.Message
		hasRunCommand := false

		for _, tc := range assistantMsg.ToolCalls {
			switch tc.Function.Name {
			case "run_command":
				var args struct {
					Command string `json:"command"`
				}
				if err := ollama.ParseToolArguments(tc.Function.Arguments, &args); err == nil {
					if os.Getenv("AI_YOLO") == "true" {
						fmt.Printf("\n> Running: %s...\n", args.Command)
						output, _ := RunCommand(args.Command)
						if output != "" {
							fmt.Printf("```\n%s\n```\n", strings.TrimSpace(output))
						}
						toolResponses = append(toolResponses, ollama.Message{
							Role:       "tool",
							ToolCallID: tc.ID,
							Content:    output,
						})
					} else {
						fmt.Printf("\n> Skip: %s (use --yolo to run in non-interactive mode)\n", args.Command)
						toolResponses = append(toolResponses, ollama.Message{
							Role:       "tool",
							ToolCallID: tc.ID,
							Content:    "Skipped: non-interactive mode without --yolo",
						})
					}
					hasRunCommand = true
				}
			case "web_search":
				var args struct {
					Query string `json:"query"`
				}
				if err := ollama.ParseToolArguments(tc.Function.Arguments, &args); err == nil {
					fmt.Printf("\n> Searching: %s...\n", args.Query)
					output, err := BraveSearch(args.Query)
					if err != nil {
						output = fmt.Sprintf("Error: %v", err)
					}
					fmt.Printf("\n%s\n", output)
					toolResponses = append(toolResponses, ollama.Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    output,
					})
					hasRunCommand = true
				}
			case "chrome_cdp":
				var args struct {
					URL    string `json:"url"`
					Action string `json:"action"`
				}
				if err := ollama.ParseToolArguments(tc.Function.Arguments, &args); err == nil {
					fmt.Printf("\n> Browsing: %s (%s)...\n", args.URL, args.Action)
					output, err := ChromeCDP(args.URL, args.Action)
					if err != nil {
						output = fmt.Sprintf("Error: %v", err)
					}
					fmt.Printf("\n%s\n", output)
					toolResponses = append(toolResponses, ollama.Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    output,
					})
					hasRunCommand = true
				}
			case "remember":
				var args struct {
					Info string `json:"info"`
				}
				if err := ollama.ParseToolArguments(tc.Function.Arguments, &args); err == nil {
					cfg, _ := config.Load()
					if cfg == nil {
						cfg = &config.Config{}
					}
					cfg.Memory = append(cfg.Memory, args.Info)
					_ = cfg.Save()

					fmt.Printf("\n> Remembered: %s\n", args.Info)

					toolResponses = append(toolResponses, ollama.Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    "Memory saved",
					})
					hasRunCommand = true
				}
			case "set_system_prompt":
				var args struct {
					Prompt string `json:"prompt"`
				}
				if err := ollama.ParseToolArguments(tc.Function.Arguments, &args); err == nil {
					cfg, _ := config.Load()
					if cfg == nil {
						cfg = &config.Config{}
					}
					cfg.SystemPrompt = args.Prompt
					_ = cfg.Save()

					fmt.Printf("\n> System prompt updated\n")

					toolResponses = append(toolResponses, ollama.Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    "System prompt updated",
					})
					hasRunCommand = true
				}
			}
		}

		if !hasRunCommand {
			fmt.Println()
			return preparedCmd, nil
		}

		// Append tool responses and loop
		req.Messages = append(req.Messages, toolResponses...)
	}
}
