package ollama

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
	go client.StreamWorker(reqBody, ch)

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
