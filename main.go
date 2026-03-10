package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/matst80/go-ai-cli/pkg/config"
	"github.com/matst80/go-ai-cli/pkg/ollama"
	"github.com/matst80/go-ai-cli/pkg/sessions"
	"github.com/matst80/go-ai-cli/pkg/terminal"
	"github.com/matst80/go-ai-cli/pkg/ui"
	"github.com/mattn/go-isatty"
)

func filter[T any](s []T, predicate func(T) bool) []T {
	result := make([]T, 0, len(s)) // Pre-allocate for efficiency
	for _, v := range s {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	prompt, images, err := terminal.ProcessInputs(flag.Args())
	if err != nil {
		fmt.Printf("Error processing input: %v\n", err)
		os.Exit(1)
	}

	if prompt == "" && len(images) == 0 {
		if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
			fmt.Println("Usage: ai [--cdp <url/port>] [--style <style>] [--yolo] [--thinking] [--url <url>] [--model <model>] <prompt> [files...]")
			os.Exit(0)
		}

		ui.InitClipboard()
		m := ui.NewInputModel()
		p := tea.NewProgram(m)
		result, err := p.Run()
		if err != nil {
			fmt.Printf("Error running interactive input: %v\n", err)
			os.Exit(1)
		}

		inputModel := result.(ui.InputModel)
		if inputModel.WasAborted() || (inputModel.Value() == "" && len(inputModel.AttachedImages()) == 0) {
			os.Exit(0)
		}

		prompt = inputModel.Value()
		images = inputModel.AttachedImages()
	}

	client := ollama.NewClient(cfg.URL)

	tools := []ollama.Tool{
		{
			Type: "function",
			Function: ollama.Function{
				Name:        "execute",
				Description: "Run a command",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "The command string to run.",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		{
			Type: "function",
			Function: ollama.Function{
				Name:        "browser",
				Description: "Control local browser and interact with pages",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"url": map[string]interface{}{
							"type":        "string",
							"description": "The URL to visit (optional for actions on current page).",
						},
						"action": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"scrape", "screenshot", "navigate", "click", "type", "scroll", "evaluate", "view_ax_tree"},
							"description": "The action to perform. 'view_ax_tree' returns interactive elements in a table. Use 'value' to filter.",
						},
						"selector": map[string]interface{}{
							"type":        "string",
							"description": "CSS selector for click, type, or scroll.",
						},
						"value": map[string]interface{}{
							"type":        "string",
							"description": "Text to type, JS to evaluate, or scroll direction: 'up', 'down', 'top', 'bottom', 'pageUp', 'pageDown'.",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: ollama.Function{
				Name:        "remember",
				Description: "Save information to persistent memory",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"info": map[string]interface{}{
							"type":        "string",
							"description": "The information to remember for future sessions.",
						},
					},
					"required": []string{"info"},
				},
			},
		},
	}

	if os.Getenv("BRAVE_API_KEY") != "" {
		tools = append(tools, ollama.Tool{
			Type: "function",
			Function: ollama.Function{
				Name:        "web_search",
				Description: "Search the internet.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "The search query.",
						},
						"country": map[string]interface{}{
							"type":        "string",
							"description": "The country code for the search (e.g. 'us', 'gb', 'se', 'de'). Optional.",
						},
						"count": map[string]interface{}{
							"type":        "integer",
							"description": "The number of results to return (max 20). Optional.",
						},
						"offset": map[string]interface{}{
							"type":        "integer",
							"description": "The zero-based offset for pagination. Optional.",
						},
					},
					"required": []string{"query"},
				},
			},
		})
	}

	systemPrompt := cfg.SystemPrompt
	if len(cfg.Memory) > 0 {
		systemPrompt += "\n\n" + strings.Join(cfg.Memory, "\n")
	}

	messages := []ollama.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	if cfg.Resume != "" {
		session, err := sessions.LoadSession(cfg.Resume)
		if err == nil && session != nil {
			messages = append(messages, session.Messages...)
			cfg.Resume = session.ID // Ensure we save back to the same ID if "last" was used
			fmt.Printf("Resumed session %s\n", session.ID)
		} else {
			fmt.Printf("Failed to resume session: %v\n", err)
		}
	}

	messages = append(messages, ollama.Message{
		Role:    "user",
		Content: prompt,
		Images:  images,
	})

	reqBody := ollama.ChatRequest{
		Model:    cfg.Model,
		Messages: messages,
		Tools:    tools,
		Stream:   true,
		Think:    cfg.Thinking,
		Options:  cfg.ModelOptions,
	}

	if !isatty.IsTerminal(os.Stdout.Fd()) {
		cmd, finalMessages, err := terminal.RunSimpleSession(client, reqBody)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		if cfg.SaveSession {
			id, err := sessions.SaveSession(cfg.Resume, finalMessages)
			if err != nil {
				fmt.Printf("\nError saving session: %v\n", err)
			} else {
				fmt.Printf("\n💾 Session saved as: %s\n", id)
			}
		}

		if cmd != "" {
			terminal.HandleSuggestedCommand(cmd)
		}
		return
	}

	uiModel := ui.NewUI(client, reqBody)
	p := tea.NewProgram(uiModel, tea.WithMouseCellMotion())
	uiModel.SetSender(p.Send)

	// Start logic loop
	go uiModel.RunInteractiveSession()

	res, err := p.Run()
	if err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}

	if finalModel, ok := res.(*ui.UI); ok {
		savedFiles := finalModel.GetSavedFiles()
		content := finalModel.GetContent()

		savedTempFiles := filter(savedFiles, func(f terminal.SavedFile) bool {
			return f.IsTemp
		})

		if len(savedTempFiles) > 0 {
			// Run file viewer with the response content as a tab
			viewer := ui.NewFileViewer(content, finalModel.GetReasoning(), savedFiles)
			vp := tea.NewProgram(viewer, tea.WithAltScreen())
			if _, err := vp.Run(); err != nil {
				fmt.Printf("Error running file viewer: %v\n", err)
			}
			// If there's an error but we showed the viewer, the error might not be visible.
			// Print it after the viewer exits.
			if finalModel.GetError() != nil {
				fmt.Println(finalModel.View())
			}
		} else if content != "" || finalModel.GetError() != nil {
			// Just show content regular if no files or if there is an error to display
			fmt.Print("\n" + finalModel.FullView())
		}

		if cfg.SaveSession {
			id, err := sessions.SaveSession(cfg.Resume, finalModel.GetMessages())
			if err != nil {
				fmt.Printf("\nError saving session: %v\n", err)
			} else {
				fmt.Printf("\n💾 Session saved as: %s\n", id)
			}
		}
	}
}
