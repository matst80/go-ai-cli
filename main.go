package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/matst80/go-ai-cli/pkg/ollama"
	"github.com/matst80/go-ai-cli/pkg/terminal"
	"github.com/mattn/go-isatty"
)

func main() {
	cdpFlag := flag.String("cdp", "", "Remote CDP URL or port (e.g. 9222 or localhost:9222)")
	flag.Parse()

	if *cdpFlag != "" {
		os.Setenv("CHROME_REMOTE_URL", *cdpFlag)
	}

	prompt := strings.Join(flag.Args(), " ")
	if prompt == "" {
		fmt.Println("Usage: ai [--cdp <url/port>] <prompt>")
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
				Description: "Add a terminal command. Use this for the final command you recommend, explore first",
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
				Description: "Run a shell command. Use this to explore or run tests before giving a final answer. (dont run destructive commands like rm -rf)",
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
				Description: "Run a browser task using CDP. Use this for scraping pages or interacting with JS-heavy sites.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"url": map[string]interface{}{
							"type":        "string",
							"description": "The URL to visit.",
						},
						"action": map[string]interface{}{
							"type":        "string",
							"description": "The action to perform (e.g., 'scrape', 'screenshot', 'navigate').",
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
				Description: "Search the web.",
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

	systemPrompt := fmt.Sprintf("You are a terminal expert on %s. Be concise, prefer one-liners. Use markdown for code blocks and formatting. Use run_command to explore local files and run tests. ", runtime.GOOS)
	if os.Getenv("BRAVE_API_KEY") != "" {
		systemPrompt += "Use web_search to find information on the web. "
	}
	systemPrompt += "Use chrome_cdp to control the browser."

	reqBody := ollama.ChatRequest{
		Model: "ministral-3:latest",
		Messages: []ollama.Message{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Tools:  tools,
		Stream: true,
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
