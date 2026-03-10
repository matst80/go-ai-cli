package ui

import (
	"encoding/base64"
	"fmt"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.design/x/clipboard"
)

type InputModel struct {
	Textarea textarea.Model
	Images   []string
	Err      error
	Quitting bool
	Aborted  bool
	Title    string
	Prompt   string
}

func NewInputModel() InputModel {
	ti := textarea.New()
	ti.Placeholder = "Enter your prompt here..."
	ti.Focus()
	ti.CharLimit = 0
	ti.SetWidth(80)
	ti.SetHeight(10)

	return InputModel{
		Textarea: ti,
		Images:   []string{},
		Err:      nil,
		Quitting: false,
		Aborted:  false,
		Title:    "Type your prompt:",
		Prompt:   "ctrl+s / ctrl+enter to submit • esc/ctrl+c to cancel • ctrl+v to paste",
	}
}

func (m InputModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m InputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			m.Quitting = true
			m.Aborted = true
			return m, tea.Quit
		case tea.KeyCtrlD, tea.KeyCtrlS, tea.KeyCtrlCaret, tea.KeyCtrlJ:
			m.Quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			if msg.Alt {
				m.Quitting = true
				return m, tea.Quit
			}
		case tea.KeyCtrlV:
			text := clipboard.Read(clipboard.FmtText)
			if len(text) > 0 {
				m.Textarea.InsertString(string(text))
			} else {
				img := clipboard.Read(clipboard.FmtImage)
				if len(img) > 0 {
					m.Images = append(m.Images, base64.StdEncoding.EncodeToString(img))
				}
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.Textarea.SetWidth(msg.Width - 4)
	}

	m.Textarea, cmd = m.Textarea.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m InputModel) View() string {
	if m.Quitting {
		return ""
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	var imgInfo string
	if len(m.Images) > 0 {
		imgInfo = fmt.Sprintf("\nAttached %d image(s)", len(m.Images))
	}

	return fmt.Sprintf(
		"%s\n\n%s\n\n%s%s\n",
		m.Title,
		m.Textarea.View(),
		helpStyle.Render(m.Prompt),
		helpStyle.Render(imgInfo),
	)
}

func (m InputModel) Value() string {
	return m.Textarea.Value()
}

func (m InputModel) AttachedImages() []string {
	return m.Images
}

func (m InputModel) WasAborted() bool {
	return m.Aborted
}

// InitClipboard initializes the clipboard package.
func InitClipboard() {
	err := clipboard.Init()
	if err != nil {
		// Just log or ignore since we don't want to crash if clipboard is unavailable.
		// fmt.Fprintf(os.Stderr, "Warning: failed to initialize clipboard: %v\n", err)
	}
}
