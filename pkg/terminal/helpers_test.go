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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractCommandFromMarkdown(tt.content); got != tt.want {
				t.Errorf("ExtractCommandFromMarkdown() = %v, want %v", got, tt.want)
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
