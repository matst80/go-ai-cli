package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/matst80/go-ai-cli/pkg/ollama"
	"github.com/matst80/go-ai-cli/pkg/terminal"
	"github.com/mattn/go-isatty"
)

func main() {
	cdpFlag := flag.String("cdp", "", "Remote CDP URL or port (e.g. 9222 or localhost:9222)")
	styleFlag := flag.String("style", "", "Output style (dark, light, or auto)")
	yoloFlag := flag.Bool("yolo", false, "Run all commands without confirmation")
	flag.Parse()

	if *yoloFlag {
		os.Setenv("AI_YOLO", "true")
	}

	if *styleFlag != "" {
		os.Setenv("AI_STYLE", *styleFlag)
	}

	if *cdpFlag != "" {
		os.Setenv("CHROME_REMOTE_URL", *cdpFlag)
	}

	prompt, images, err := terminal.ProcessInputs(flag.Args())
	if err != nil {
		fmt.Printf("Error processing input: %v\n", err)
		os.Exit(1)
	}

	if prompt == "" && len(images) == 0 {
		fmt.Println("Usage: ai [--cdp <url/port>] [--style <style>] [--yolo] <prompt> [files...]")
		os.Exit(1)
	}
	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434/api/chat"
	}

	client := ollama.NewClient(ollamaURL)

	tools := []ollama.Tool{
		{
			Type: "function",
			Function: ollama.Function{
				Name:        "add_command",
				Description: "Suggest a terminal command",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "The command string to add to the terminal.",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		{
			Type: "function",
			Function: ollama.Function{
				Name:        "run_command",
				Description: "Run a shell command",
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
				Name:        "chrome_cdp",
				Description: "Control a browser",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"url": map[string]interface{}{
							"type":        "string",
							"description": "The URL to visit.",
						},
						"action": map[string]interface{}{
							"type":        "string",
							"description": "The action to perform",
							"enum":        []string{"scrape", "screenshot", "navigate"},
						},
					},
					"required": []string{"url", "action"},
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
					},
					"required": []string{"query"},
				},
			},
		})
	}

	osName := runtime.GOOS
	// if osName == "darwin" {
	// 	osName = "macOS"
	// }

	systemPrompt := fmt.Sprintf("You are a terminal expert for %s. find a way to help the user", osName)

	ollamaModel := os.Getenv("OLLAMA_MODEL")
	if ollamaModel == "" {
		ollamaModel = "ministral-3:latest"
	}

	reqBody := ollama.ChatRequest{
		Model: ollamaModel,
		Messages: []ollama.Message{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: prompt,
				Images:  images,
			},
		},
		Tools:  tools,
		Stream: true,
		Think:  true,
		Options: map[string]interface{}{
			"temperature": 0,
			"num_ctx":     8192, // Ensure enough room for long tool-calling sessions
		},
	}

	if !isatty.IsTerminal(os.Stdout.Fd()) {
		cmd, err := terminal.RunSimpleSession(client, reqBody)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		if cmd != "" {
			terminal.HandleSuggestedCommand(cmd)
		}
		return
	}

	ui := terminal.NewUI(client, reqBody)
	p := tea.NewProgram(ui)

	// Start logic loop
	go ui.RunInteractiveSession()

	// Initial trigger for Bubble Tea
	go func() {
		p.Send(<-ui.ChunkChan())
	}()

	res, err := p.Run()
	if err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}

	if finalModel, ok := res.(*terminal.UI); ok {
		if finalModel.GetError() != nil {
			fmt.Printf("\nError: %v\n", finalModel.GetError())
		} else if finalModel.GetContent() != "" {
			fmt.Print("\n" + finalModel.View())
		}

		cmd := finalModel.GetPreparedCmd()
		if cmd == "" && !finalModel.ToolWasCalled() && finalModel.GetContent() != "" {
			cmd = terminal.ExtractCommandFromMarkdown(finalModel.GetContent())
		}

		if cmd != "" {
			terminal.HandleSuggestedCommand(cmd)
		}
	}
}
