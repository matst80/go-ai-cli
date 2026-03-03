package terminal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type SavedFile struct {
	Path    string
	Content string
	IsTemp  bool
}

type FileViewer struct {
	response string
	files    []SavedFile
	cursor   int
	viewport viewport.Model
	width    int
	height   int
	ready    bool
}

func NewFileViewer(response string, files []SavedFile) *FileViewer {
	return &FileViewer{
		response: response,
		files:    files,
	}
}

func (m *FileViewer) Init() tea.Cmd {
	return nil
}

func (m *FileViewer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "up", "k", "left", "h", "shift+tab":
			if m.cursor > 0 {
				m.cursor--
				m.updateViewport()
			}
		case "down", "j", "right", "l", "tab":
			if m.cursor < len(m.files) {
				m.cursor++
				m.updateViewport()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 3
		footerHeight := 1
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}

		m.updateViewport()
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *FileViewer) updateViewport() {
	var content string
	var lang string

	if m.cursor == 0 {
		content = m.response
	} else {
		f := m.files[m.cursor-1]
		ext := filepath.Ext(f.Path)
		if len(ext) > 1 {
			lang = ext[1:]
		}
		content = fmt.Sprintf("```%s\n%s\n```", lang, f.Content)
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

	r, _ := glamour.NewTermRenderer(
		styleOpt,
		glamour.WithWordWrap(m.width-4),
	)

	rendered, _ := r.Render(content)
	m.viewport.SetContent(rendered)
	m.viewport.GotoTop()
}

func (m *FileViewer) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	var header strings.Builder

	// Response tab
	respTab := " Response "
	if m.cursor == 0 {
		respTab = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("255")).
			Bold(true).
			Render(respTab)
	} else {
		respTab = "  Response  "
	}
	header.WriteString(respTab)

	// File tabs
	for i, f := range m.files {
		path := filepath.Base(f.Path)
		if f.IsTemp {
			path = "temp: " + path
		}

		item := path
		if i+1 == m.cursor {
			item = lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("255")).
				Bold(true).
				Render(" " + item + " ")
		} else {
			item = "  " + item + "  "
		}
		header.WriteString(item)
	}

	line := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(strings.Repeat("─", m.width))

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(fmt.Sprintf(" %d/%d • q: quit • arrows/hjkl/tab: navigate", m.cursor+1, len(m.files)+1))

	return fmt.Sprintf("%s\n%s\n%s\n%s",
		header.String(),
		line,
		m.viewport.View(),
		footer)
}
