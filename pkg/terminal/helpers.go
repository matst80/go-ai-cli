package terminal

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/mattn/go-isatty"
)

// RunCommand executes a shell command and returns output with a 60-second timeout
func RunCommand(command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(output) + "\nError: Command timed out after 60s", ctx.Err()
	}
	return string(output), err
}

// ExtractCommandFromMarkdown searches for bash code blocks in markdown text
func ExtractCommandFromMarkdown(content string) string {
	// Look for triple backtick blocks: ```bash, ```sh, or just ```
	lines := strings.Split(content, "\n")
	inBlock := false
	var blockContent []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if !inBlock {
				inBlock = true
				continue
			} else {
				// End of block
				if len(blockContent) > 0 {
					return strings.TrimSpace(strings.Join(blockContent, "\n"))
				}
				inBlock = false
			}
		}
		if inBlock {
			blockContent = append(blockContent, line)
		}
	}
	return ""
}

// HandleSuggestedCommand presents a command for the user to run
func HandleSuggestedCommand(cmd string) {
	if isatty.IsTerminal(os.Stdin.Fd()) && runtime.GOOS != "windows" {
		// Small delay to let the shell prompt reappear if any
		time.Sleep(150 * time.Millisecond)
		for _, c := range []byte(cmd) {
			_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, os.Stdin.Fd(), syscall.TIOCSTI, uintptr(unsafe.Pointer(&c)))
		}
	}
	fmt.Printf("\nSuggested command: %s\n", cmd)
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
