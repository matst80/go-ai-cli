package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/matst80/go-ai-cli/pkg/config"
	"github.com/matst80/go-ai-cli/pkg/ollama"
	"github.com/matst80/go-ai-cli/pkg/terminal"
	"github.com/mattn/go-isatty"
)

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
		fmt.Println("Usage: ai [--cdp <url/port>] [--style <style>] [--yolo] [--url <url>] [--model <model>] <prompt> [files...]")
		os.Exit(1)
	}

	client := ollama.NewClient(cfg.URL)

	tools := []ollama.Tool{
		{
			Type: "function",
			Function: ollama.Function{
				Name:        "run",
				Description: "Run a shell command, dont use it to write files",
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

	reqBody := ollama.ChatRequest{
		Model: cfg.Model,
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
			"temperature": 0.5,
			"num_ctx":     16384, // Ensure enough room for long tool-calling sessions
		},
	}

	if cfg != nil && cfg.ModelOptions != nil {
		for k, v := range cfg.ModelOptions {
			reqBody.Options[k] = v
		}
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
