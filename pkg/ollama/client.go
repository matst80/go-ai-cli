package ollama

import (
	"bufio"
	"bytes"
	"context"
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
func (c *Client) StreamWorker(ctx context.Context, req ChatRequest, ch chan StreamResponse) {
	defer close(ch)
	LogDebug("--- NEW REQUEST ---")
	jsonData, err := json.Marshal(req)
	if err != nil {
		LogDebug("Marshal error: %v", err)
		ch <- StreamResponse{Error: fmt.Errorf("failed to marshal request: %w", err)}
		return
	}
	LogDebug("Payload (%d bytes): %s", len(jsonData), string(jsonData))

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		LogDebug("Request creation error: %v", err)
		ch <- StreamResponse{Error: err}
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			LogDebug("Request cancelled")
			return
		}
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
		select {
		case <-ctx.Done():
			LogDebug("Context cancelled during read")
			return
		default:
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			if ctx.Err() != nil {
				return
			}
			LogDebug("Read error: %v", err)
			ch <- StreamResponse{Error: err}
			return
		}

		cleanLine := bytes.TrimSpace(line)
		if len(cleanLine) == 0 {
			continue
		}

		LogDebug("got line: %s", string(cleanLine))
		var chatResp ChatResponse
		if err := json.Unmarshal(cleanLine, &chatResp); err != nil {
			LogDebug("Unmarshal error: %v, line: %s", err, string(cleanLine))
			continue
		}

		if chatResp.Error != "" {
			LogDebug("Ollama error field: %s", chatResp.Error)
			ch <- StreamResponse{Error: fmt.Errorf("ollama error: %s", chatResp.Error)}
			continue
		}

		if len(chatResp.Message.ToolCalls) > 0 {
			for i := range chatResp.Message.ToolCalls {
				if chatResp.Message.ToolCalls[i].Type == "" {
					chatResp.Message.ToolCalls[i].Type = "function"
				}
			}
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
}
