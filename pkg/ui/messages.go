package ui

import "github.com/matst80/go-ai-cli/pkg/ollama"

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

type combinedRenderMsg struct {
	Reasoning         string
	Content           string
	ReasoningDirty    bool
	ContentDirty      bool
	OriginalContent   string
	OriginalReasoning string
}
