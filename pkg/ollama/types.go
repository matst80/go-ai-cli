package ollama

import (
	"encoding/json"
	"fmt"

	"github.com/matst80/go-ai-cli/pkg/config"
)

// Message represents a chat message in the Ollama API
type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	Images           []string   `json:"images,omitempty"`
	ReasoningContent string     `json:"thinking,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
}

// Tool represents a tool that the AI can use
type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function represents the function definition for a tool
type Function struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// ToolCall represents a call to a tool from the AI
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}

// ChatRequest represents the request body for Ollama chat API
type ChatRequest struct {
	Model    string              `json:"model"`
	Messages []Message           `json:"messages"`
	Stream   bool                `json:"stream"`
	Think    bool                `json:"think,omitempty"`
	Tools    []Tool              `json:"tools,omitempty"`
	Options  config.ModelOptions `json:"options,omitempty"`
}

// ChatResponse represents the response chunk
type ChatResponse struct {
	Model   string  `json:"model"`
	Message Message `json:"message"`
	Done    bool    `json:"done"`
	Error   string  `json:"error,omitempty"`
}

// ParseToolArguments handles both object and stringified JSON arguments
func ParseToolArguments(data json.RawMessage, target interface{}) error {
	// Try direct unmarshal (if it's an object)
	if err := json.Unmarshal(data, target); err == nil {
		return nil
	}
	// Try unmarshal as string then as JSON (if it's a stringified JSON)
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if err := json.Unmarshal([]byte(s), target); err == nil {
			return nil
		} else {
			return fmt.Errorf("could not parse stringified tool arguments: %w, raw: %s", err, string(s))
		}
	}
	return fmt.Errorf("could not parse tool arguments: %s", string(data))
}
