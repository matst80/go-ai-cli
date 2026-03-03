package terminal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
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
	response  string
	files     []SavedFile
	cursor    int
	viewport  viewport.Model
	width     int
	height    int
	ready     bool
	cache     map[int]string
	input     textinput.Model
	showing   bool
	err       error
	clipboard string
}

func NewFileViewer(response string, files []SavedFile) *FileViewer {
	ti := textinput.New()
	ti.Placeholder = "Enter filename..."
	return &FileViewer{
		response: response,
		files:    files,
		cache:    make(map[int]string),
		input:    ti,
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

	if m.showing {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				if m.input.Value() != "" {
					f := &m.files[m.cursor-1]
					newPath := m.input.Value()
					dir := filepath.Dir(newPath)
					if dir != "." {
						if err := os.MkdirAll(dir, 0755); err != nil {
							m.err = err
							m.showing = false
							return m, nil
						}
					}
					if err := os.WriteFile(newPath, []byte(f.Content), 0644); err != nil {
						m.err = err
						m.showing = false
						return m, nil
					}
					f.Path = newPath
					f.IsTemp = false
					m.showing = false
					m.err = nil
					m.input.SetValue("")
					delete(m.cache, m.cursor)
					m.updateViewport()
					return m, nil
				}
			case "esc":
				m.showing = false
				m.input.SetValue("")
				return m, nil
			}
		}
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "left", "h", "shift+tab", "[":
			if m.cursor > 0 {
				m.cursor--
				m.clipboard = ""
				m.updateViewport()
				return m, nil
			}
		case "right", "l", "tab", "]":
			if m.cursor < len(m.files) {
				m.cursor++
				m.clipboard = ""
				m.updateViewport()
				return m, nil
			}
		case "s":
			if m.cursor > 0 {
				m.showing = true
				m.input.Focus()
				m.input.SetValue(filepath.Base(m.files[m.cursor-1].Path))
				return m, textinput.Blink
			}
		case "c":
			var text string
			if m.cursor == 0 {
				text = m.response
			} else {
				text = m.files[m.cursor-1].Content
			}
			CopyToClipboard(text)
			m.clipboard = "Copied to clipboard!"
			return m, nil
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(msg.String()[0] - '1')
			if idx <= len(m.files) {
				m.cursor = idx
				m.clipboard = ""
				m.updateViewport()
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.cache = make(map[int]string) // Invalidate cache on resize

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
		return m, nil
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *FileViewer) updateViewport() {
	if rendered, ok := m.cache[m.cursor]; ok {
		m.viewport.SetContent(rendered)
		m.viewport.GotoTop()
		return
	}

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

	width := m.width
	if width < 10 {
		width = 80 // fallback
	}

	r, _ := glamour.NewTermRenderer(
		styleOpt,
		glamour.WithWordWrap(width-4),
	)

	rendered, _ := r.Render(content)
	m.cache[m.cursor] = rendered
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
		Render(fmt.Sprintf(" %d/%d • q: quit • tab/h/l: tabs • j/k: scroll • s: save • c: copy • 1-9: switch", m.cursor+1, len(m.files)+1))

	view := fmt.Sprintf("%s\n%s\n%s\n%s",
		header.String(),
		line,
		m.viewport.View(),
		footer)

	if m.clipboard != "" {
		view += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(" "+m.clipboard)
	}

	if m.showing {
		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1).
			Width(m.width / 2)

		inputView := style.Render(fmt.Sprintf("Save As:\n\n%s", m.input.View()))

		// Overlay input in center
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, inputView)
	}

	if m.err != nil {
		errStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Render(fmt.Sprintf(" Error: %v ", m.err))
		return view + "\n" + errStyle
	}

	return view
}
