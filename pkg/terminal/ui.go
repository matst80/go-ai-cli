package terminal

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/matst80/go-ai-cli/pkg/ollama"
)

// UI represents the tool's Bubble Tea model and methods
type UI struct {
	content       string
	err           error
	renderer      *glamour.TermRenderer
	client        *ollama.Client
	request       ollama.ChatRequest
	done          bool
	width         int
	preparedCmd   string
	toolWasCalled bool
	chunkChan     chan tea.Msg
	confirmCh     chan bool
	confirmCmd    string
}

// Msg types for Bubble Tea
type responseMsg string
type toolCallMsg []ollama.ToolCall
type errorMsg error
type doneMsg bool
type confirmationMsg struct {
	Command string
	Ch      chan bool
}

// NewUI creates a new UI model
func NewUI(client *ollama.Client, req ollama.ChatRequest) *UI {
	return &UI{
		client:    client,
		request:   req,
		chunkChan: make(chan tea.Msg),
	}
}

func (u *UI) Init() tea.Cmd {
	return nil
}

func (u *UI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return u, tea.Quit
		}
		if u.confirmCh != nil {
			if msg.String() == "y" || msg.String() == "Y" {
				u.confirmCh <- true
				u.confirmCh = nil
				u.confirmCmd = ""
			} else if msg.String() == "n" || msg.String() == "N" {
				u.confirmCh <- false
				u.confirmCh = nil
				u.confirmCmd = ""
			}
		}
	case tea.WindowSizeMsg:
		u.width = msg.Width
		u.renderer = nil // Recreate renderer on size change
	case responseMsg:
		u.content += string(msg)
		return u, u.waitForNextChunk()
	case toolCallMsg:
		u.toolWasCalled = true
		for _, tc := range msg {
			if tc.Function.Name == "add_command" {
				var args struct {
					Command string `json:"command"`
				}
				if err := ollama.ParseToolArguments(tc.Function.Arguments, &args); err == nil && args.Command != "" {
					u.preparedCmd = args.Command
				}
			}
		}
		return u, u.waitForNextChunk()
	case confirmationMsg:
		u.confirmCh = msg.Ch
		u.confirmCmd = msg.Command
		return u, u.waitForNextChunk()
	case errorMsg:
		u.err = msg
		return u, tea.Quit
	case doneMsg:
		u.done = true
		return u, tea.Quit
	}
	return u, nil
}

func (u *UI) View() string {
	if u.err != nil {
		return fmt.Sprintf("\nError: %v\n", u.err)
	}

	if u.content == "" {
		status := " Thinking..."
		if u.toolWasCalled {
			status = " Preparing command..."
		}
		return status
	}

	if u.renderer == nil {
		w := u.width
		if w == 0 {
			w = 80
		}

		style := os.Getenv("AI_STYLE")
		if style == "" {
			style = os.Getenv("GLAMOUR_STYLE")
		}

		var styleOpt glamour.TermRendererOption
		if style != "" && style != "auto" {
			styleOpt = glamour.WithStandardStyle(style)
		} else {
			styleOpt = glamour.WithAutoStyle()
		}

		u.renderer, _ = glamour.NewTermRenderer(
			styleOpt,
			glamour.WithWordWrap(w-2),
		)
	}

	out, _ := u.renderer.Render(u.content)
	if u.confirmCh != nil {
		out += fmt.Sprintf("\n> Run command: `%s`? [y/N] ", u.confirmCmd)
	}
	return out
}

func (u *UI) waitForNextChunk() tea.Cmd {
	return func() tea.Msg {
		return <-u.chunkChan
	}
}

// RunInteractiveSession handles tool execution loops for a TTY
func (u *UI) RunInteractiveSession() {
	for {
		workerCh := make(chan ollama.StreamResponse)
		go u.client.StreamWorker(u.request, workerCh)

		var assistantMsg ollama.Message
		assistantMsg.Role = "assistant"

		for msg := range workerCh {
			if msg.Error != nil {
				u.chunkChan <- errorMsg(msg.Error)
				return
			}
			if msg.Content != "" {
				assistantMsg.Content += msg.Content
				u.chunkChan <- responseMsg(msg.Content)
			}
			if len(msg.ToolCalls) > 0 {
				assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, msg.ToolCalls...)
				u.chunkChan <- toolCallMsg(msg.ToolCalls)
			}
		}

		// Add assistant message to history
		u.request.Messages = append(u.request.Messages, assistantMsg)

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
					shouldRun := os.Getenv("AI_YOLO") == "true"
					if !shouldRun {
						ch := make(chan bool)
						u.chunkChan <- confirmationMsg{Command: args.Command, Ch: ch}
						shouldRun = <-ch
						if !shouldRun {
							u.chunkChan <- responseMsg(fmt.Sprintf("\n> Skip: `%s`...\n", args.Command))
						}
					}

					var output string
					if shouldRun {
						u.chunkChan <- responseMsg(fmt.Sprintf("\n> Running: `%s`...\n", args.Command))
						output, _ = RunCommand(args.Command)
						if output != "" {
							u.chunkChan <- responseMsg(fmt.Sprintf("```\n%s\n```\n", strings.TrimSpace(output)))
						}
					}

					toolResponses = append(toolResponses, ollama.Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    output,
					})
					hasRunCommand = true
				}
			case "web_search":
				var args struct {
					Query string `json:"query"`
				}
				if err := ollama.ParseToolArguments(tc.Function.Arguments, &args); err == nil {
					u.chunkChan <- responseMsg(fmt.Sprintf("\n> Searching: `%s`...\n", args.Query))
					output, err := BraveSearch(args.Query)
					if err != nil {
						output = fmt.Sprintf("Error: %v", err)
					}
					u.chunkChan <- responseMsg(fmt.Sprintf("\n%s\n", output))

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
					u.chunkChan <- responseMsg(fmt.Sprintf("\n> Browsing: %s (%s)...\n", args.URL, args.Action))
					output, err := ChromeCDP(args.URL, args.Action)
					if err != nil {
						output = fmt.Sprintf("Error: %v", err)
					}
					u.chunkChan <- responseMsg(fmt.Sprintf("\n%s\n", output))

					toolResponses = append(toolResponses, ollama.Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    output,
					})
					hasRunCommand = true
				}
			}
		}

		if !hasRunCommand {
			u.chunkChan <- doneMsg(true)
			return
		}

		// Append tool responses and loop for more chain-of-thought
		u.request.Messages = append(u.request.Messages, toolResponses...)
	}
}

// GetResult methods for after the program runs
func (u *UI) GetPreparedCmd() string  { return u.preparedCmd }
func (u *UI) GetContent() string      { return u.content }
func (u *UI) GetError() error         { return u.err }
func (u *UI) ToolWasCalled() bool     { return u.toolWasCalled }
func (u *UI) ChunkChan() chan tea.Msg { return u.chunkChan }
