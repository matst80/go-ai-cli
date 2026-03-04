package terminal

import (
	"os"
	"strings"
	"testing"
)

func TestExtractCommandFromMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "simple sh block",
			content: "Here is a command:\n```sh\nls -la\n```",
			want:    "ls -la",
		},
		{
			name:    "bash block",
			content: "Try this:\n```bash\necho 'hello'\n```",
			want:    "echo 'hello'",
		},
		{
			name:    "no language specified",
			content: "```\npwd\n```",
			want:    "pwd",
		},
		{
			name:    "no code block",
			content: "Just some text",
			want:    "",
		},
		{
			name:    "ignore html block",
			content: "```html\n<div></div>\n```",
			want:    "",
		},
		{
			name:    "last block wins",
			content: "```bash\nfirst\n```\n```bash\nsecond\n```",
			want:    "second",
		},
		{
			name:    "ignore mixed blocks",
			content: "```bash\ncmd\n```\n```html\nnot a cmd\n```",
			want:    "cmd",
		},
		{
			name:    "block with filename",
			content: "```bash:script.sh\necho hi\n```",
			want:    "echo hi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractCommandFromMarkdown(tt.content); got != tt.want {
				t.Errorf("ExtractCommandFromMarkdown() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "already has https",
			url:  "https://google.com",
			want: "https://google.com",
		},
		{
			name: "already has http",
			url:  "http://example.com",
			want: "http://example.com",
		},
		{
			name: "missing scheme",
			url:  "www.elgiganten.se",
			want: "https://www.elgiganten.se",
		},
		{
			name: "empty url",
			url:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeURL(tt.url); got != tt.want {
				t.Errorf("NormalizeURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcessInputs(t *testing.T) {
	// Create a dummy text file
	content := "hello world"
	tmpFile, err := os.CreateTemp("", "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	prompt, images, err := ProcessInputs([]string{"analyze", tmpFile.Name()})
	if err != nil {
		t.Fatalf("ProcessInputs failed: %v", err)
	}

	if !strings.Contains(prompt, "analyze") {
		t.Errorf("prompt doesn't contain 'analyze': %s", prompt)
	}
	if !strings.Contains(prompt, "--- File: ") {
		t.Errorf("prompt doesn't contain file header: %s", prompt)
	}
	if !strings.Contains(prompt, content) {
		t.Errorf("prompt doesn't contain file content: %s", prompt)
	}
	if len(images) != 0 {
		t.Errorf("expected 0 images, got %d", len(images))
	}
}
