package terminal

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/matst80/go-ai-cli/pkg/config"
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

// escapeHTMLOutsideCodeBlocks escapes '<' to prevent markdown parser from eating unknown HTML tags
func escapeHTMLOutsideCodeBlocks(content string) string {
	var result strings.Builder
	inCodeBlock := false

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			result.WriteString(line)
			if i < len(lines)-1 {
				result.WriteString("\n")
			}
			continue
		}

		if inCodeBlock {
			result.WriteString(line)
			if i < len(lines)-1 {
				result.WriteString("\n")
			}
			continue
		}

		inInlineCode := false
		var processed strings.Builder
		chars := []rune(line)
		for j := 0; j < len(chars); j++ {
			if chars[j] == '`' {
				inInlineCode = !inInlineCode
				processed.WriteRune(chars[j])
			} else if chars[j] == '<' && !inInlineCode {
				processed.WriteString("\\<")
			} else {
				processed.WriteRune(chars[j])
			}
		}

		result.WriteString(processed.String())
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}
	return result.String()
}

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
	spinner       spinner.Model
	savedFiles    []SavedFile
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
type fileSavedMsg struct {
	Filename string
	Content  string
	IsTemp   bool
}
type commandMsg string

// NewUI creates a new UI model
func NewUI(client *ollama.Client, req ollama.ChatRequest) *UI {
	ctx, cancel := context.WithCancel(context.Background())
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return &UI{
		client:     client,
		request:    req,
		chunkChan:  make(chan tea.Msg),
		ctx:        ctx,
		cancel:     cancel,
		spinner:    s,
		savedFiles: make([]SavedFile, 0),
	}
}

func (u *UI) Init() tea.Cmd {
	return u.spinner.Tick
}

func (u *UI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		u.spinner, cmd = u.spinner.Update(msg)
		return u, cmd
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
		return u, u.waitForNextChunk()
	case confirmationMsg:
		u.confirmCh = msg.Ch
		u.confirmCmd = msg.Command
		return u, u.waitForNextChunk()
	case errorMsg:
		u.err = msg
		return u, tea.Quit
	case fileSavedMsg:
		u.savedFiles = append(u.savedFiles, SavedFile{
			Path:    msg.Filename,
			Content: msg.Content,
			IsTemp:  msg.IsTemp,
		})
		return u, u.waitForNextChunk()
	case commandMsg:
		u.preparedCmd = string(msg)
		CopyToClipboard(string(msg))
		return u, u.waitForNextChunk()
	case tea.MouseMsg:
		if msg.Type == tea.MouseLeft && u.confirmCh != nil {
			// Basic click detection for [y/N]
			// This is just to show mouse support is active
		}
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
		status := fmt.Sprintf(" %s Thinking...", u.spinner.View())
		if u.toolWasCalled {
			status = fmt.Sprintf(" %s Preparing command...", u.spinner.View())
		}
		out = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(status)
	} else {
		displayContent := u.content
		if u.reasoning != "" {
			displayContent = "_Thinking..._\n" + u.reasoning + "\n\n---\n\n" + u.content
		}

		// Ensure code blocks are closed for partial rendering so they show up properly while streaming
		if strings.Count(displayContent, "```")%2 != 0 {
			displayContent += "\n```"
		}

		displayContent = escapeHTMLOutsideCodeBlocks(displayContent)

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

		// Post-process markers for saved files
		for _, f := range u.savedFiles {
			marker := fmt.Sprintf("@@SAVED:%s@@", f.Path)
			if strings.Contains(rendered, marker) {
				rendered = strings.ReplaceAll(rendered, marker, u.renderFileSavedLines(f.Path))
			}
		}

		out = rendered

		if !u.done && u.confirmCh == nil {
			out += fmt.Sprintf("\n %s\n", u.spinner.View())
		}
	}

	if u.confirmCh != nil {
		choices := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("[y/N]")
		prompt := fmt.Sprintf("⚡ %s%s\n%s\n\n%s ",
			headerStyle.Render("Run command:"),
			infoStyle.Render(u.confirmCmd),
			lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true).Render("(copied to clipboard)"),
			choices)

		promptView := "\n" + borderStyle.Render(prompt) + "\n"
		if u.content == "" && u.reasoning == "" {
			// If we're just showing status, replace it
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

		sh := NewStreamHandler(
			func(text string) { u.chunkChan <- responseMsg(text) },
			func(filename, content string, isTemp bool) {
				u.chunkChan <- fileSavedMsg{Filename: filename, Content: content, IsTemp: isTemp}
			},
			func(cmd string) {
				u.chunkChan <- commandMsg(cmd)
			},
		)

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
				sh.Feed(msg.Content)
			}
			if len(msg.ToolCalls) > 0 {
				assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, msg.ToolCalls...)
				u.chunkChan <- toolCallMsg(msg.ToolCalls)
			}
		}
		sh.Flush()

		// Add assistant message to history
		u.request.Messages = append(u.request.Messages, assistantMsg)

		// Check for tool calls that need immediate execution
		var toolResponses []ollama.Message
		hasRunCommand := false

		for _, tc := range assistantMsg.ToolCalls {
			switch tc.Function.Name {
			case "execute":
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

					u.chunkChan <- responseMsg(fmt.Sprintf("\n**Remembered:** %s\n", args.Info))

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

					u.chunkChan <- responseMsg("\n**System prompt updated**\n")

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
			u.chunkChan <- doneMsg(true)
			return
		}

		// Append tool responses and loop for more chain-of-thought
		u.request.Messages = append(u.request.Messages, toolResponses...)
	}
}

func (u *UI) renderFileSavedLines(filename string) string {
	var f SavedFile
	for _, sf := range u.savedFiles {
		if sf.Path == filename {
			f = sf
			break
		}
	}
	content := f.Content
	size := len(content)

	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	sizeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	header := fmt.Sprintf("💾 %s %s %s",
		titleStyle.Render("SAVED"),
		pathStyle.Render(filename),
		sizeStyle.Render(fmt.Sprintf("(%d bytes)", size)))

	var preview string
	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		maxLines := 8
		if len(lines) > maxLines {
			preview = strings.Join(lines[:maxLines], "\n") + "\n" + sizeStyle.Render("...")
		} else {
			preview = strings.Join(lines, "\n")
		}
		// Dim the preview
		preview = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(preview)
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("42")).
		Padding(0, 1).
		MarginBottom(1).
		Render(header + "\n\n" + preview)

	return "\n" + box + "\n"
}

// GetResult methods for after the program runs
func (u *UI) GetPreparedCmd() string { return u.preparedCmd }
func (u *UI) GetContent() string     { return u.content }
func (u *UI) GetError() error        { return u.err }
func (u *UI) ToolWasCalled() bool    { return u.toolWasCalled }
func (u *UI) GetSavedFiles() []SavedFile {
	return u.savedFiles
}

func (u *UI) ChunkChan() chan tea.Msg { return u.chunkChan }
