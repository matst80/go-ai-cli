package terminal

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
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
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true)
)

// escapeHTMLOutsideCodeBlocks escapes '<' to prevent markdown parser from eating unknown HTML tags
func escapeHTMLOutsideCodeBlocks(content string) string {
	var result strings.Builder
	inCodeBlock := false
	inInlineCode := false

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
		} else if !inCodeBlock {
			line = processInlineCode(line, &inInlineCode)
		}

		result.WriteString(line)
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}
	return result.String()
}

func processInlineCode(line string, inInlineCode *bool) string {
	var processed strings.Builder
	chars := []rune(line)
	for j := 0; j < len(chars); j++ {
		if chars[j] == '`' {
			*inInlineCode = !*inInlineCode
		} else if chars[j] == '<' && !*inInlineCode {
			processed.WriteString("\\<")
			continue
		}
		processed.WriteRune(chars[j])
	}
	return processed.String()
}

// UI represents the tool's Bubble Tea model and methods
type UI struct {
	content           string
	reasoning         string
	err               error
	renderer          Renderer
	fs                FileService
	client            ChatClient
	request           ollama.ChatRequest
	done              bool
	width             int
	preparedCmd       string
	toolWasCalled     bool
	summarizing       bool
	Send              func(tea.Msg)
	confirmCh         chan bool
	confirmCmd        string
	ctx               context.Context
	cancel            context.CancelFunc
	spinner           spinner.Model
	savedFiles        []SavedFile
	askMoreInput      bool
	inputMode         bool
	inputModel        InputModel
	moreInputCh       chan string
	height            int
	rReasoning        Renderer
	rContent          Renderer
	lastWReasoning    int
	lastWContent      int
	renderedReasoning string
	renderedContent   string
	styledReasoning   string
	styledContent     string
	combinedView      string
	reasoningDirty    bool
	contentDirty      bool
	isActive          bool
	isRendering       bool
	clipboardMsg      string
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
type summarizingMsg bool
type moreInputMsg chan string
type busyMsg bool

// NewUI creates a new UI model
func NewUI(client ChatClient, renderer Renderer, fs FileService, req ollama.ChatRequest) *UI {
	ctx, cancel := context.WithCancel(context.Background())
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &UI{
		client:      client,
		request:     req,
		renderer:    renderer,
		fs:          fs,
		ctx:         ctx,
		cancel:      cancel,
		spinner:     s,
		summarizing: false,
		savedFiles:  make([]SavedFile, 0),
		isActive:    true,
	}
}

func (u *UI) SetSender(send func(tea.Msg)) {
	u.Send = send
}

func (u *UI) Init() tea.Cmd {
	return tea.Batch(u.spinner.Tick, u.renderBackground())
}

func (u *UI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		return u.handleTickMsg(msg)
	case tea.KeyMsg:
		return u.handleKeyMsg(msg)
	case tea.WindowSizeMsg:
		return u.handleWindowSizeMsg(msg)
	case responseMsg:
		return u.handleResponseMsg(msg)
	case reasoningMsg:
		return u.handleReasoningMsg(msg)
	case combinedRenderMsg:
		return u.handleCombinedRenderMsg(msg)
	case busyMsg:
		return u.handleBusyMsg(msg)
	case toolCallMsg:
		return u.handleToolCallMsg(msg)
	case confirmationMsg:
		return u.handleConfirmationMsg(msg)
	case errorMsg:
		return u.handleErrorMsg(msg)
	case fileSavedMsg:
		return u.handleFileSavedMsg(msg)
	case commandMsg:
		return u.handleCommandMsg(msg)
	case summarizingMsg:
		return u.handleSummarizingMsg(msg)
	case tea.MouseMsg:
		return u, nil
	case doneMsg:
		return u, tea.Quit
	case moreInputMsg:
		return u.handleMoreInputMsg(msg)
	}
	return u, nil
}

func (u *UI) handleTickMsg(msg spinner.TickMsg) (tea.Model, tea.Cmd) {
	if !u.isActive {
		return u, nil
	}
	var cmd tea.Cmd
	u.spinner, cmd = u.spinner.Update(msg)
	return u, cmd
}

func (u *UI) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc {
		u.cancel()
		return u, tea.Quit
	}

	if u.confirmCh != nil {
		return u.handleConfirmInput(msg.String())
	}
	if u.askMoreInput {
		return u.handleAskMoreInput(msg)
	}
	if u.inputMode {
		return u.handleInputMode(msg)
	}

	if msg.String() == "c" {
		CopyToClipboard(u.content)
		u.clipboardMsg = "Full conversation copied to clipboard!"
	}
	return u, nil
}

func (u *UI) handleConfirmInput(input string) (tea.Model, tea.Cmd) {
	if input == "y" || input == "Y" {
		u.confirmCh <- true
	} else if input == "n" || input == "N" {
		u.confirmCh <- false
	}
	u.confirmCh = nil
	u.confirmCmd = ""
	return u, nil
}

func (u *UI) handleAskMoreInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	input := msg.String()
	if input == "y" || input == "Y" {
		u.askMoreInput = false
		u.inputMode = true
		u.inputModel = NewInputModel()
		u.inputModel.Title = "Follow-up prompt:"
		u.inputModel.Prompt = "ctrl+s / ctrl+j / alt+enter to submit • esc/ctrl+c to cancel"
		u.inputModel.Textarea.SetWidth(u.width - 10)
		return u, u.inputModel.Init()
	}
	if input == "n" || input == "N" || msg.Type == tea.KeyEnter {
		u.askMoreInput = false
		ch := u.moreInputCh
		u.moreInputCh = nil
		go func() { ch <- "" }()
		return u, nil
	}
	if input == "c" {
		CopyToClipboard(u.content)
		u.clipboardMsg = "Full conversation copied to clipboard!"
	}
	return u, nil
}

func (u *UI) handleInputMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc {
		u.inputMode = false
		ch := u.moreInputCh
		u.moreInputCh = nil
		go func() { ch <- "" }()
		return u, nil
	}

	m, cmd := u.inputModel.Update(msg)
	u.inputModel = m.(InputModel)
	if u.inputModel.Quitting {
		u.inputMode = false
		ch := u.moreInputCh
		u.moreInputCh = nil
		go func() { ch <- u.inputModel.Value() }()
		return u, u.spinner.Tick
	}
	return u, cmd
}

func (u *UI) handleWindowSizeMsg(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	u.width = msg.Width
	u.height = msg.Height
	u.renderer = nil
	u.rReasoning = nil
	u.rContent = nil
	u.reasoningDirty = true
	u.contentDirty = true
	return u, nil
}

func (u *UI) handleResponseMsg(msg responseMsg) (tea.Model, tea.Cmd) {
	u.content += string(msg)
	u.contentDirty = true
	return u, u.renderBackground()
}

func (u *UI) handleReasoningMsg(msg reasoningMsg) (tea.Model, tea.Cmd) {
	u.reasoning += string(msg)
	u.reasoningDirty = true
	return u, u.renderBackground()
}

func (u *UI) handleCombinedRenderMsg(msg combinedRenderMsg) (tea.Model, tea.Cmd) {
	if msg.ReasoningDirty {
		u.renderedReasoning = msg.Reasoning
		if len(u.reasoning) == len(msg.OriginalReasoning) {
			u.reasoningDirty = false
		}

		// Limit to last 7 lines
		displayReasoning := u.renderedReasoning
		maxReasoningLines := 7
		count := 0
		lastIdx := 0
		found := false
		for i := len(displayReasoning) - 1; i >= 0; i-- {
			if displayReasoning[i] == '\n' {
				count++
				if count >= maxReasoningLines {
					lastIdx = i + 1
					found = true
					break
				}
			}
		}
		if found {
			displayReasoning = "...\n" + displayReasoning[lastIdx:]
		}

		// Update styled reasoning
		reasoningStyle := lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(lipgloss.Color("240")).
			Foreground(lipgloss.Color("245")).
			MaxHeight(maxReasoningLines + 2). // +2 for border and ellipsis
			MarginTop(1)
		u.styledReasoning = reasoningStyle.Render(displayReasoning)
	}
	if msg.ContentDirty {
		u.renderedContent = msg.Content
		if len(u.content) == len(msg.OriginalContent) {
			u.contentDirty = false
		}
	}

	// Update combined view (re-calculate whenever reasoning or content changes)
	if msg.ReasoningDirty || msg.ContentDirty {
		if u.reasoning != "" {
			u.combinedView = lipgloss.JoinVertical(lipgloss.Left,
				u.renderedContent,
				u.styledReasoning,
			)
		} else {
			u.combinedView = u.renderedContent
		}
	}

	u.isRendering = false
	if u.reasoningDirty || u.contentDirty {
		return u, u.renderBackground()
	}
	return u, nil
}

func (u *UI) handleBusyMsg(msg busyMsg) (tea.Model, tea.Cmd) {
	u.isActive = bool(msg)
	if u.isActive {
		return u, u.spinner.Tick
	}
	return u, nil
}

func (u *UI) handleToolCallMsg(msg toolCallMsg) (tea.Model, tea.Cmd) {
	u.toolWasCalled = true
	return u, nil
}

func (u *UI) handleConfirmationMsg(msg confirmationMsg) (tea.Model, tea.Cmd) {
	u.confirmCh = msg.Ch
	u.confirmCmd = msg.Command
	return u, nil
}

func (u *UI) handleErrorMsg(msg errorMsg) (tea.Model, tea.Cmd) {
	u.err = msg
	u.done = true
	return u, tea.Quit
}

func (u *UI) handleFileSavedMsg(msg fileSavedMsg) (tea.Model, tea.Cmd) {
	u.savedFiles = append(u.savedFiles, SavedFile{
		Path:    msg.Filename,
		Content: msg.Content,
		IsTemp:  msg.IsTemp,
	})
	return u, nil
}

func (u *UI) handleCommandMsg(msg commandMsg) (tea.Model, tea.Cmd) {
	u.preparedCmd = string(msg)
	CopyToClipboard(string(msg))
	return u, nil
}

func (u *UI) handleSummarizingMsg(msg summarizingMsg) (tea.Model, tea.Cmd) {
	u.summarizing = bool(msg)
	return u, nil
}

func (u *UI) handleMoreInputMsg(msg moreInputMsg) (tea.Model, tea.Cmd) {
	u.moreInputCh = msg
	u.askMoreInput = true
	u.isActive = false
	return u, nil
}

func (u *UI) renderBackground() tea.Cmd {
	if u.isRendering || (!u.reasoningDirty && !u.contentDirty) {
		return nil
	}
	u.isRendering = true

	w := u.width
	if w == 0 {
		w = 80
	}

	style := os.Getenv("AI_STYLE")
	if style == "" {
		style = os.Getenv("GLAMOUR_STYLE")
	}

	// Re-use the injected renderer, or create one if nil
	if u.renderer == nil {
		u.renderer = NewDefaultRenderer(style)
	}

	// Since we are sharing a single injected u.renderer, we shouldn't have separate rContent and rReasoning instances in the struct.
	// But glamour renderer is slow to recreate on every render if we just toggle width.
	// For full DI without a factory, we will just use `u.renderer` and accept the width toggle
	// in the render loop if both change, but to minimize changes and be completely correct,
	// we will just set the width right before we render the parts.

	// Capture state synchronously
	renderer := u.renderer
	originalContent := u.content
	originalReasoning := u.reasoning
	reasoningWasDirty := u.reasoningDirty
	contentWasDirty := u.contentDirty

	return func() tea.Msg {
		var renderedReasoning, renderedContent string
		var err error

		if reasoningWasDirty {
			if renderer != nil {
				renderer.SetWidth(w - 4)
				renderedReasoning, err = renderer.Render("_Thinking..._\n" + originalReasoning)
				if err != nil {
					renderedReasoning = "_Thinking..._\n" + originalReasoning
				}
			} else {
				renderedReasoning = "_Thinking..._\n" + originalReasoning
			}
		}

		if contentWasDirty {
			displayContent := originalContent
			if strings.Count(displayContent, "```")%2 != 0 {
				displayContent += "\n```"
			}
			displayContent = escapeHTMLOutsideCodeBlocks(displayContent)
			if renderer != nil {
				renderer.SetWidth(w - 2)
				renderedContent, err = renderer.Render(displayContent)
				if err != nil {
					renderedContent = displayContent
				}
			} else {
				renderedContent = displayContent
			}
		}

		return combinedRenderMsg{
			Reasoning:         renderedReasoning,
			Content:           renderedContent,
			ReasoningDirty:    reasoningWasDirty,
			ContentDirty:      contentWasDirty,
			OriginalContent:   originalContent,
			OriginalReasoning: originalReasoning,
		}
	}
}

type combinedRenderMsg struct {
	Reasoning         string
	Content           string
	ReasoningDirty    bool
	ContentDirty      bool
	OriginalContent   string
	OriginalReasoning string
}

func (u *UI) FullView() string {
	out := u.buildMainContent()
	out = u.appendPrompts(out)
	out = u.appendErrors(out)
	out = u.appendClipboardMsg(out)
	return out
}

func (u *UI) buildMainContent() string {
	if u.content == "" && u.reasoning == "" {
		return u.renderStatus()
	}
	return u.renderContentWithMarkers()
}

func (u *UI) renderStatus() string {
	status := fmt.Sprintf(" %s Thinking...", u.spinner.View())
	if u.toolWasCalled {
		status = fmt.Sprintf(" %s Preparing command...", u.spinner.View())
	}
	if u.summarizing {
		status = fmt.Sprintf(" %s Summarizing context...", u.spinner.View())
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(status)
}

func (u *UI) renderContentWithMarkers() string {
	if u.content == "" && u.reasoning == "" {
		return u.renderStatus()
	}

	out := u.combinedView

	for _, f := range u.savedFiles {
		marker := fmt.Sprintf("@@SAVED:%s@@", f.Path)
		if strings.Contains(out, marker) {
			out = strings.ReplaceAll(out, marker, u.renderFileSavedLines(f.Path))
		}
	}

	if u.isActive && !u.done && u.confirmCh == nil {
		out += fmt.Sprintf("\n %s\n", u.spinner.View())
	}
	return out
}

func (u *UI) appendPrompts(out string) string {
	if u.confirmCh != nil {
		return out + u.renderConfirmPrompt()
	}
	if u.askMoreInput {
		return out + u.renderAskMorePrompt()
	}
	if u.inputMode {
		return out + u.renderInputPrompt()
	}
	return out
}

func (u *UI) renderConfirmPrompt() string {
	choices := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("[y/N]")
	prompt := fmt.Sprintf("⚡ %s%s\n%s\n\n%s ",
		headerStyle.Render("Run command:"),
		infoStyle.Render(u.confirmCmd),
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true).Render("(copied to clipboard)"),
		choices+lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(" • c: copy all"))

	promptView := "\n" + borderStyle.Render(prompt) + "\n"
	if u.content == "" && u.reasoning == "" {
		return promptView
	}
	return promptView
}

func (u *UI) renderAskMorePrompt() string {
	prompt := fmt.Sprintf("💬 %s %s",
		headerStyle.Render("More input?"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("[y/N] • c: copy all"))
	return "\n" + borderStyle.Render(prompt) + "\n"
}

func (u *UI) renderInputPrompt() string {
	return "\n" + borderStyle.Render(u.inputModel.View()) + "\n"
}

func (u *UI) appendErrors(out string) string {
	if u.err != nil {
		return out + fmt.Sprintf("\n%s\n", errorStyle.Render(fmt.Sprintf("Error: %v", u.err)))
	}
	return out
}

func (u *UI) appendClipboardMsg(out string) string {
	if u.clipboardMsg != "" {
		return out + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(" "+u.clipboardMsg)
	}
	return out
}

func (u *UI) View() string {
	out := u.FullView()

	if u.height > 2 && !u.done {
		// Find the start of the last u.height - 1 lines without splitting the whole string
		maxLines := u.height - 1
		count := 0
		lastIdx := len(out)
		for i := len(out) - 1; i >= 0; i-- {
			if out[i] == '\n' {
				count++
				if count >= maxLines {
					lastIdx = i + 1
					break
				}
			}
		}
		if count >= maxLines {
			out = out[lastIdx:]
		}
	}
	return out
}

// RunInteractiveSession handles tool execution loops for a TTY
func (u *UI) RunInteractiveSession() {
	for {
		select {
		case <-u.ctx.Done():
			u.Send(doneMsg(true))
			return
		default:
		}

		u.Send(busyMsg(true))
		u.Send(summarizingMsg(true))
		ManageContext(u.ctx, u.client, &u.request)
		u.Send(summarizingMsg(false))

		workerCh := make(chan ollama.StreamResponse)
		go u.client.StreamWorker(u.ctx, u.request, workerCh)

		var assistantMsg ollama.Message
		assistantMsg.Role = "assistant"

		sh := NewStreamHandler(
			u.fs,
			func(text string) { u.Send(responseMsg(text)) },
			func(filename, content string, isTemp bool) {
				u.Send(fileSavedMsg{Filename: filename, Content: content, IsTemp: isTemp})
			},
			func(cmd string) {
				u.Send(commandMsg(cmd))
			},
		)

		// Throttling for reasoning updates
		var reasoningBuf strings.Builder
		lastReasoningSend := time.Now()

		for msg := range workerCh {
			if msg.Error != nil {
				u.Send(errorMsg(msg.Error))
				return
			}
			if msg.ReasoningContent != "" {
				assistantMsg.ReasoningContent += msg.ReasoningContent
				reasoningBuf.WriteString(msg.ReasoningContent)

				// Send reasoning updates at most every 50ms or if buffer is large
				if time.Since(lastReasoningSend) > 50*time.Millisecond || reasoningBuf.Len() > 500 {
					u.Send(reasoningMsg(reasoningBuf.String()))
					reasoningBuf.Reset()
					lastReasoningSend = time.Now()
				}
			}
			if msg.Content != "" {
				assistantMsg.Content += msg.Content
				sh.Feed(msg.Content)
			}
			if len(msg.ToolCalls) > 0 {
				assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, msg.ToolCalls...)
				u.Send(toolCallMsg(msg.ToolCalls))
			}
		}
		// Flush remaining reasoning
		if reasoningBuf.Len() > 0 {
			u.Send(reasoningMsg(reasoningBuf.String()))
		}
		sh.Flush()

		// Add assistant message to history
		u.request.Messages = append(u.request.Messages, assistantMsg)

		// Check for tool calls that need immediate execution
		var toolResponses []ollama.Message
		hasRunCommand := false
		executor := NewToolExecutor()
		uiHandler := &uiToolHandler{
			ui: u,
		}

		for _, tc := range assistantMsg.ToolCalls {
			output, images, err := executor.HandleToolCall(u.ctx, tc, uiHandler)
			if err != nil {
				// Handle specific parse errors or unknown tools gracefully
				continue
			}

			toolResponses = append(toolResponses, ollama.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    output,
				Images:     images,
			})
			hasRunCommand = true
		}

		if !hasRunCommand {
			u.Send(busyMsg(false))
			moreCh := make(chan string)
			u.Send(moreInputMsg(moreCh))
			newInput := <-moreCh
			if newInput == "" {
				u.Send(doneMsg(true))
				return
			}
			u.request.Messages = append(u.request.Messages, ollama.Message{
				Role:    "user",
				Content: newInput,
			})
			u.content += "\n\n---\n\n**YOU:** " + newInput + "\n\n"
			u.reasoning = ""
			continue
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
func (u *UI) GetReasoning() string   { return u.reasoning }
func (u *UI) GetError() error        { return u.err }
func (u *UI) ToolWasCalled() bool    { return u.toolWasCalled }
func (u *UI) GetSavedFiles() []SavedFile {
	return u.savedFiles
}
func (u *UI) GetMessages() []ollama.Message {
	return u.request.Messages
}

// uiToolHandler implements ToolUI for the Bubble Tea interactive UI
type uiToolHandler struct {
	ui *UI
}

func (h *uiToolHandler) ConfirmCommand(cmd string) bool {
	shouldRun := os.Getenv("AI_YOLO") == "true"
	if !shouldRun {
		ch := make(chan bool)
		h.ui.Send(confirmationMsg{Command: cmd, Ch: ch})
		select {
		case shouldRun = <-ch:
		case <-h.ui.ctx.Done():
			return false
		}
		if !shouldRun {
			h.ui.Send(responseMsg(fmt.Sprintf("\n_Skip: %s_\n", cmd)))
		}
	}
	return shouldRun
}

func (h *uiToolHandler) LogActivity(activity string) {
	h.ui.Send(responseMsg(fmt.Sprintf("\n%s\n", activity)))
}

func (h *uiToolHandler) LogOutput(output string) {
	if output != "" {
		if strings.HasPrefix(output, "```") {
			h.ui.Send(responseMsg(fmt.Sprintf("%s\n", output)))
		} else if strings.Contains(output, "\n") {
			h.ui.Send(responseMsg(fmt.Sprintf("\n```\n%s\n```\n", strings.TrimSpace(output))))
		} else {
			h.ui.Send(responseMsg(fmt.Sprintf("\n%s\n", output)))
		}
	}
}
