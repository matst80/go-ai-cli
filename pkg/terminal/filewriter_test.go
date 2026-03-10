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
		nil,
		func(text string) {},
		func(filename, content string, isTemp bool) { savedFile = filename },
		func(cmd string) {},
	)

	sh.Feed("```html\n")
	sh.Feed("<html>\n")
	sh.Feed("<body>hi</body>\n")
	sh.Feed("</html>\n")
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
	expected := "<html>\n<body>hi</body>\n</html>"
	if string(data) != expected {
		t.Errorf("unexpected content: %q, expected %q", string(data), expected)
	}
}

func TestStreamHandler_ExplicitFile(t *testing.T) {
	var savedFile string
	sh := NewStreamHandler(
		nil,
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
		nil,
		func(text string) {},
		func(filename, content string, isTemp bool) { savedFile = filename },
		func(cmd string) { suggestedCmd = cmd },
	)

	sh.Feed("```bash\n")
	sh.Feed("echo hello\n")
	sh.Feed("ls -la\n")
	sh.Feed("```\n")
	sh.Flush()

	if !strings.HasSuffix(savedFile, ".bash") {
		t.Errorf("expected .bash temp file, got %q", savedFile)
	}
	expectedCmd := "echo hello\nls -la"
	if suggestedCmd != expectedCmd {
		t.Errorf("expected %q, got %q", expectedCmd, suggestedCmd)
	}
}

func TestStreamHandler_OneLinerSkip(t *testing.T) {
	var savedFile string
	var suggestedCmd string
	sh := NewStreamHandler(
		nil,
		func(text string) {},
		func(filename, content string, isTemp bool) { savedFile = filename },
		func(cmd string) { suggestedCmd = cmd },
	)

	// One-liner bash: should suggest command but NOT save file
	sh.Feed("```bash\n")
	sh.Feed("echo only-one-line\n")
	sh.Feed("```\n")
	sh.Flush()

	if savedFile != "" {
		t.Errorf("expected no file to be saved for one-liner, got %q", savedFile)
	}
	if suggestedCmd != "echo only-one-line" {
		t.Errorf("expected command to be suggested even if not saved")
	}

	// One-liner html: should NOT save file
	savedFile = ""
	sh.Feed("```html\n")
	sh.Feed("<html></html>\n")
	sh.Feed("```\n")
	sh.Flush()

	if savedFile != "" {
		t.Errorf("expected no file to be saved for one-liner html")
	}
}
