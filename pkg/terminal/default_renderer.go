package terminal

import (
	"os"

	"github.com/charmbracelet/glamour"
)

// DefaultRenderer provides rendering for markdown via glamour
type DefaultRenderer struct {
	style string
	width int
	inner *glamour.TermRenderer
}

// NewDefaultRenderer creates a new DefaultRenderer
func NewDefaultRenderer(style string) *DefaultRenderer {
	if style == "" {
		style = os.Getenv("AI_STYLE")
		if style == "" {
			style = os.Getenv("GLAMOUR_STYLE")
		}
	}
	r := &DefaultRenderer{style: style, width: 80}
	r.recreateRenderer()
	return r
}

func (r *DefaultRenderer) recreateRenderer() {
	var styleOpt glamour.TermRendererOption
	if r.style != "" && r.style != "auto" {
		styleOpt = glamour.WithStandardStyle(r.style)
	} else {
		styleOpt = glamour.WithAutoStyle()
	}

	renderer, err := glamour.NewTermRenderer(styleOpt, glamour.WithWordWrap(r.width))
	if err == nil {
		r.inner = renderer
	}
}

// Render renders the provided markdown string
func (r *DefaultRenderer) Render(in string) (string, error) {
	if r.inner == nil {
		return in, nil
	}
	rendered, err := r.inner.Render(in)
	if err != nil {
		return in, err
	}
	return rendered, nil
}

// SetWidth updates the renderer's output width and recreates the internal glamour renderer if needed
func (r *DefaultRenderer) SetWidth(width int) {
	if width <= 0 {
		width = 80
	}
	if r.width != width {
		r.width = width
		r.recreateRenderer()
	}
}
