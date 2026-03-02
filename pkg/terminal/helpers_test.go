package terminal

import (
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
