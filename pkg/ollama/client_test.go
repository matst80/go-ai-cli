package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/matst80/go-ai-cli/pkg/config"
)

func TestClient_StreamWorker(t *testing.T) {
	// Mock Ollama server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")

		responses := []ChatResponse{
			{Message: Message{Content: "Hello "}, Done: false},
			{Message: Message{Content: "there!"}, Done: true},
		}

		for _, resp := range responses {
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	reqBody := ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "user", Content: "hi"},
		},
		Stream: true,
	}

	ch := make(chan StreamResponse)
	go client.StreamWorker(context.Background(), reqBody, ch)

	var content string
	for resp := range ch {
		if resp.Error != nil {
			t.Fatalf("StreamWorker failed: %v", resp.Error)
		}
		content += resp.Content
		if resp.Done {
			break
		}
	}

	expected := "Hello there!"
	if content != expected {
		t.Errorf("got %q, want %q", content, expected)
	}
}

func TestClient_StreamWorker_Cancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		// Delay to allow cancellation to happen
		time.Sleep(100 * time.Millisecond)
		json.NewEncoder(w).Encode(ChatResponse{Message: Message{Content: "unseen"}})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan StreamResponse)

	go client.StreamWorker(ctx, ChatRequest{}, ch)
	cancel()

	for resp := range ch {
		if resp.Content == "unseen" {
			t.Error("received content after cancellation")
		}
	}
}

func TestParseToolArguments(t *testing.T) {
	type args struct {
		Command string `json:"command"`
	}

	tests := []struct {
		name    string
		data    string
		want    string
		wantErr bool
	}{
		{
			name: "object arguments",
			data: `{"command": "ls -la"}`,
			want: "ls -la",
		},
		{
			name: "stringified arguments",
			data: `"{\"command\": \"ls -la\"}"`,
			want: "ls -la",
		},
		{
			name:    "invalid arguments",
			data:    `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got args
			err := ParseToolArguments(json.RawMessage(tt.data), &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseToolArguments() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Command != tt.want {
				t.Errorf("ParseToolArguments() got = %v, want %v", got.Command, tt.want)
			}
		})
	}
}
func TestChatRequest_Marshal(t *testing.T) {
	req := ChatRequest{
		Model:   "test",
		Options: config.ModelOptions{RepeatPenalty: 1.5},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	options, ok := decoded["options"].(map[string]interface{})
	if !ok {
		t.Fatalf("options not found or not a map")
	}
	if options["repeat_penalty"] != 1.5 {
		t.Errorf("got repeat_penalty %v, want 1.5", options["repeat_penalty"])
	}
}
