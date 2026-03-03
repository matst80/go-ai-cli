package terminal

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/mattn/go-isatty"
)

// RunCommand executes a shell command and returns output with a 60-second timeout
func RunCommand(command string) (string, error) {
	if command == "" {
		return "no command provided?", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash")
	cmd.Stdin = strings.NewReader(command)

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(output) + "\nError: Command timed out after 60s", ctx.Err()
	}
	return string(output), err
}

// CreateFile creates a file with the given name and content
func CreateFile(filename, content string) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filename, []byte(content), 0644)
}

// AppendFile appends content to an existing file
func AppendFile(filename, content string) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

// EditFile replaces a specific block of text in a file
func EditFile(filename, search, replace string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	content := string(data)
	if !strings.Contains(content, search) {
		return fmt.Errorf("search string not found in file")
	}
	// For safety, we only replace the FIRST occurrence to avoid accidental mass changes
	// if the search string is too generic.
	newContent := strings.Replace(content, search, replace, 1)
	return os.WriteFile(filename, []byte(newContent), 0644)
}

// ExtractCommandFromMarkdown searches for the LAST bash/sh code block in markdown text
func ExtractCommandFromMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	inBlock := false
	var blockContent []string
	var lastCommand string
	var currentLang string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, ":::") {
			prefix := "```"
			if strings.HasPrefix(trimmed, ":::") {
				prefix = ":::"
			}

			if !inBlock {
				inBlock = true
				tag := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
				if idx := strings.Index(tag, ":"); idx != -1 {
					currentLang = strings.TrimSpace(tag[:idx])
				} else {
					currentLang = tag
				}
				blockContent = nil
				continue
			} else {
				// End of block
				if (currentLang == "bash" || currentLang == "sh" || currentLang == "") && len(blockContent) > 0 {
					lastCommand = strings.TrimSpace(strings.Join(blockContent, "\n"))
				}
				inBlock = false
			}
		} else if inBlock {
			blockContent = append(blockContent, line)
		}
	}
	return lastCommand
}

// CopyToClipboard uses the OSC 52 escape sequence to copy text to the system clipboard.
// This is supported by many modern terminals (iTerm2, Alacritty, Kitty, VSCode, etc).
func CopyToClipboard(text string) {
	if text == "" {
		return
	}
	// OSC 52: \033]52;c;<base64>\007
	fmt.Fprintf(os.Stderr, "\033]52;c;%s\007", base64.StdEncoding.EncodeToString([]byte(text)))
}

// HandleSuggestedCommand presents a command for the user to run
func HandleSuggestedCommand(cmd string) {
	if isatty.IsTerminal(os.Stdin.Fd()) && runtime.GOOS != "windows" {
		// Also copy to clipboard for convenience
		CopyToClipboard(cmd)

		// Small delay to let the shell prompt reappear if any
		time.Sleep(150 * time.Millisecond)
		for _, c := range []byte(cmd) {
			_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, os.Stdin.Fd(), syscall.TIOCSTI, uintptr(unsafe.Pointer(&c)))
		}
	}
	fmt.Printf("\nSuggested command: %s (copied to clipboard)\n", cmd)
}

// ProcessInputs handles piped stdin and file arguments, returning a prompt, images, and any error
func ProcessInputs(args []string) (string, []string, error) {
	var images []string
	var extraContent []string
	var promptParts []string

	// Check if stdin is piped
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		data, err := io.ReadAll(os.Stdin)
		if err == nil && len(data) > 0 {
			mimeType := http.DetectContentType(data)
			if strings.HasPrefix(mimeType, "image/") {
				images = append(images, base64.StdEncoding.EncodeToString(data))
			} else {
				extraContent = append(extraContent, string(data))
			}
		}
	}

	for _, arg := range args {
		// Only treat as file if it exists and is not a directory
		if info, err := os.Stat(arg); err == nil && !info.IsDir() {
			data, err := os.ReadFile(arg)
			if err == nil {
				mimeType := http.DetectContentType(data)
				if strings.HasPrefix(mimeType, "image/") {
					images = append(images, base64.StdEncoding.EncodeToString(data))
				} else {
					extraContent = append(extraContent, fmt.Sprintf("\n--- File: %s ---\n%s\n", arg, string(data)))
				}
				continue
			}
		}
		promptParts = append(promptParts, arg)
	}

	prompt := strings.Join(promptParts, " ")
	if len(extraContent) > 0 {
		if prompt != "" {
			prompt += "\n"
		}
		prompt += strings.Join(extraContent, "\n")
	}

	return prompt, images, nil
}
