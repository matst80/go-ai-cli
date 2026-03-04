package terminal

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/matst80/go-ai-cli/pkg/ollama"
)

// RunSimpleSession provides a non-interactive output for non-TTY or fallback
func RunSimpleSession(client *ollama.Client, req ollama.ChatRequest) (string, []ollama.Message, error) {
	var preparedCmd string
	for {
		summarized, _ := ManageContext(context.Background(), client, &req)
		if summarized {
			fmt.Println("\n> Summarizing context...")
		}

		workerCh := make(chan ollama.StreamResponse)
		go client.StreamWorker(context.Background(), req, workerCh)

		var assistantMsg ollama.Message
		assistantMsg.Role = "assistant"

		sh := NewStreamHandler(
			func(text string) { fmt.Print(text) },
			func(filename, content string, isTemp bool) {
				if isTemp {
					fmt.Printf("\n💾 Saved to temp: %s (%d bytes)\n", filename, len(content))
				} else {
					fmt.Printf("\n💾 Saved: %s (%d bytes)\n", filename, len(content))
				}
			},
			func(cmd string) {
				preparedCmd = cmd
			},
		)

		for msg := range workerCh {
			if msg.Error != nil {
				return "", req.Messages, msg.Error
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
					fmt.Print("\n\n----- \n\n")
				}
				assistantMsg.Content += msg.Content
				sh.Feed(msg.Content)
			}
			if len(msg.ToolCalls) > 0 {
				assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, msg.ToolCalls...)
			}
		}
		sh.Flush()

		// Add assistant message to history
		req.Messages = append(req.Messages, assistantMsg)

		// Check for tool calls that need immediate execution
		var toolResponses []ollama.Message
		hasRunCommand := false

		executor := NewToolExecutor()
		uiHandler := &simpleToolHandler{}

		for _, tc := range assistantMsg.ToolCalls {
			output, err := executor.HandleToolCall(context.Background(), tc, uiHandler)
			if err != nil {
				continue
			}
			toolResponses = append(toolResponses, ollama.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    output,
			})
			hasRunCommand = true
		}

		if !hasRunCommand {
			fmt.Println()
			return preparedCmd, req.Messages, nil
		}

		// Append tool responses and loop
		req.Messages = append(req.Messages, toolResponses...)
	}
}

// simpleToolHandler implements ToolUI for the simple non-interactive UI
type simpleToolHandler struct{}

func (h *simpleToolHandler) ConfirmCommand(cmd string) bool {
	shouldRun := os.Getenv("AI_YOLO") == "true"
	if !shouldRun {
		fmt.Printf("\n> Skip: %s (use --yolo to run in non-interactive mode)\n", cmd)
	}
	return shouldRun
}

func (h *simpleToolHandler) LogActivity(activity string) {
	// Strip markdown formatting since it's simple output
	activity = strings.ReplaceAll(activity, "**", "")
	activity = strings.ReplaceAll(activity, "`", "")
	activity = strings.ReplaceAll(activity, "*", "")
	fmt.Printf("\n> %s...\n", activity)
}

func (h *simpleToolHandler) LogOutput(output string) {
	if output != "" {
		if strings.Contains(output, "\n") && !strings.HasPrefix(output, "```") {
			fmt.Printf("```\n%s\n```\n", strings.TrimSpace(output))
		} else {
			fmt.Printf("\n%s\n", output)
		}
	}
}
