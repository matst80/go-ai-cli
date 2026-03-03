package ollama

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client handles interaction with the Ollama API
type Client struct {
	URL string
}

// NewClient creates a new Ollama client
func NewClient(url string) *Client {
	return &Client{URL: url}
}

// LogDebug writes to ai.log for troubleshooting
func LogDebug(format string, v ...interface{}) {
	f, err := os.OpenFile("ai.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, time.Now().Format("15:04:05 ")+format+"\n", v...)
}

// StreamResponse represents possible messages during streaming
type StreamResponse struct {
	Content          string
	ReasoningContent string
	ToolCalls        []ToolCall
	Error            error
	Done             bool
}

// StreamWorker handles the streaming request to Ollama
func (c *Client) StreamWorker(req ChatRequest, ch chan StreamResponse) {
	LogDebug("--- NEW REQUEST ---")
	jsonData, err := json.Marshal(req)
	if err != nil {
		LogDebug("Marshal error: %v", err)
		ch <- StreamResponse{Error: fmt.Errorf("failed to marshal request: %w", err)}
		return
	}
	LogDebug("Payload (%d bytes): %s", len(jsonData), string(jsonData))

	resp, err := http.Post(c.URL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		LogDebug("Post error: %v", err)
		ch <- StreamResponse{Error: err}
		return
	}
	defer resp.Body.Close()

	LogDebug("Response Status: %s", resp.Status)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		LogDebug("Error Body: %s", string(body))
		var chatResp ChatResponse
		if err := json.Unmarshal(body, &chatResp); err == nil && chatResp.Error != "" {
			ch <- StreamResponse{Error: fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, chatResp.Error)}
		} else {
			ch <- StreamResponse{Error: fmt.Errorf("ollama request failed with status %d: %s", resp.StatusCode, string(body))}
		}
		return
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			LogDebug("Read error: %v", err)
			ch <- StreamResponse{Error: err}
			return
		}

		LogDebug("Recv: %s", string(line))
		var chatResp ChatResponse
		if err := json.Unmarshal(line, &chatResp); err != nil {
			LogDebug("Unmarshal error: %v, line: %s", err, string(line))
			continue
		}

		if chatResp.Error != "" {
			LogDebug("Ollama error field: %s", chatResp.Error)
			ch <- StreamResponse{Error: fmt.Errorf("ollama error: %s", chatResp.Error)}
			return
		}

		if len(chatResp.Message.ToolCalls) > 0 {
			ch <- StreamResponse{ToolCalls: chatResp.Message.ToolCalls}
		}

		if chatResp.Message.ReasoningContent != "" {
			ch <- StreamResponse{ReasoningContent: chatResp.Message.ReasoningContent}
		}

		if chatResp.Message.Content != "" {
			ch <- StreamResponse{Content: chatResp.Message.Content}
		}

		if chatResp.Done {
			break
		}
	}
	ch <- StreamResponse{Done: true}
	close(ch)
}
