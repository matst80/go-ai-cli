package terminal

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/matst80/go-ai-cli/pkg/ollama"
)

var (
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("62")).
			Bold(true).
			PaddingRight(1)
	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)
	skipStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)
)

// UI represents the tool's Bubble Tea model and methods
type UI struct {
	content       string
	reasoning     string
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
	ctx           context.Context
	cancel        context.CancelFunc
}

// Msg types for Bubble Tea
type responseMsg string
type reasoningMsg string
type toolCallMsg []ollama.ToolCall
type errorMsg error
type doneMsg bool
type confirmationMsg struct {
	Command string
	Ch      chan bool
}

// NewUI creates a new UI model
func NewUI(client *ollama.Client, req ollama.ChatRequest) *UI {
	ctx, cancel := context.WithCancel(context.Background())
	return &UI{
		client:    client,
		request:   req,
		chunkChan: make(chan tea.Msg),
		ctx:       ctx,
		cancel:    cancel,
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
		if msg.Type == tea.KeyEsc {
			u.cancel()
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
	case reasoningMsg:
		u.reasoning += string(msg)
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

	var out string
	if u.content == "" && u.reasoning == "" {
		status := " Thinking..."
		if u.toolWasCalled {
			status = " Preparing command..."
		}
		out = status
	} else {
		displayContent := u.content
		if u.reasoning != "" {
			displayContent = "_Thinking..._\n" + u.reasoning + "\n\n---\n\n" + u.content
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

		rendered, _ := u.renderer.Render(displayContent)
		out = rendered
	}

	if u.confirmCh != nil {
		choices := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("[y/N]")
		prompt := fmt.Sprintf("⚡ %s%s\n\n%s ",
			headerStyle.Render("Run command:"),
			infoStyle.Render(u.confirmCmd),
			choices)

		promptView := "\n" + borderStyle.Render(prompt) + "\n"
		if out == " Preparing command..." || out == " Thinking..." {
			// If we're just showing status, replace it or prepend it
			out = promptView
		} else {
			out += promptView
		}
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
		select {
		case <-u.ctx.Done():
			u.chunkChan <- doneMsg(true)
			return
		default:
		}

		workerCh := make(chan ollama.StreamResponse)
		go u.client.StreamWorker(u.ctx, u.request, workerCh)

		var assistantMsg ollama.Message
		assistantMsg.Role = "assistant"

		for msg := range workerCh {
			if msg.Error != nil {
				u.chunkChan <- errorMsg(msg.Error)
				return
			}
			if msg.ReasoningContent != "" {
				assistantMsg.ReasoningContent += msg.ReasoningContent
				u.chunkChan <- reasoningMsg(msg.ReasoningContent)
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
						select {
						case shouldRun = <-ch:
						case <-u.ctx.Done():
							return
						}
						if !shouldRun {
							u.chunkChan <- responseMsg(fmt.Sprintf("\n_Skip: %s_\n", args.Command))
						}
					}

					var output string
					if shouldRun {
						u.chunkChan <- responseMsg(fmt.Sprintf("\n**Running:** `%s`\n", args.Command))
						output, _ = RunCommand(args.Command)
						if output != "" {
							u.chunkChan <- responseMsg(fmt.Sprintf("```\n%s\n```\n", strings.TrimSpace(output)))
						}
					} else {
						output = "Cancelled by user"
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
					u.chunkChan <- responseMsg(fmt.Sprintf("\n**Searching:** `%s`\n", args.Query))
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
					u.chunkChan <- responseMsg(fmt.Sprintf("\n**Browsing:** `%s` *(%s)*\n", args.URL, args.Action))
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
