package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStreamHandler_TempFile(t *testing.T) {
	var savedFile string
	sh := NewStreamHandler(
		func(text string) {},
		func(filename, content string, isTemp bool) { savedFile = filename },
		func(cmd string) {},
	)

	sh.Feed("```html\n")
	sh.Feed("<html><body>hi</body></html>\n")
	sh.Feed("```\n")
	sh.Flush()

	if savedFile == "" {
		t.Fatal("expected a temp file to be saved")
	}
	if !strings.Contains(savedFile, "snippet_") || !strings.HasSuffix(savedFile, ".html") {
		t.Errorf("unexpected temp filename: %q", savedFile)
	}

	// Check content
	data, _ := os.ReadFile(savedFile)
	if string(data) != "<html><body>hi</body></html>" {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestStreamHandler_ExplicitFile(t *testing.T) {
	var savedFile string
	sh := NewStreamHandler(
		func(text string) {},
		func(filename, content string, isTemp bool) { savedFile = filename },
		func(cmd string) {},
	)

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "index.html")

	sh.Feed("```html:" + target + "\n")
	sh.Feed("hi\n")
	sh.Feed("```\n")
	sh.Flush()

	if savedFile != target {
		t.Errorf("expected %q, got %q", target, savedFile)
	}
}

func TestStreamHandler_BashAndTemp(t *testing.T) {
	var savedFile string
	var suggestedCmd string
	sh := NewStreamHandler(
		func(text string) {},
		func(filename, content string, isTemp bool) { savedFile = filename },
		func(cmd string) { suggestedCmd = cmd },
	)

	sh.Feed("```bash\n")
	sh.Feed("echo hello\n")
	sh.Feed("```\n")
	sh.Flush()

	if !strings.HasSuffix(savedFile, ".bash") {
		t.Errorf("expected .bash temp file, got %q", savedFile)
	}
	if suggestedCmd != "echo hello" {
		t.Errorf("expected 'echo hello', got %q", suggestedCmd)
	}
}
