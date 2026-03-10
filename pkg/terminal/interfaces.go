package terminal

import (
	"context"

	"github.com/matst80/go-ai-cli/pkg/ollama"
)

// ChatClient defines the interface for communicating with the AI backend.
type ChatClient interface {
	GenerateResponse(ctx context.Context, req ollama.ChatRequest) (string, error)
	StreamWorker(ctx context.Context, req ollama.ChatRequest, ch chan ollama.StreamResponse)
}

// Renderer defines the interface for rendering markdown or other text formats.
type Renderer interface {
	Render(in string) (string, error)
	SetWidth(width int)
}

// FileService defines the interface for handling file system operations.
type FileService interface {
	WriteFile(filename string, data []byte, perm ...interface{}) error
	MkdirAll(path string, perm ...interface{}) error
}
