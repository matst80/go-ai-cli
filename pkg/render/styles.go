package render

import "github.com/charmbracelet/lipgloss"

var (
	HeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("62")).
			Bold(true).
			PaddingRight(1)
	InfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)
	SkipStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)
	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true)
)
