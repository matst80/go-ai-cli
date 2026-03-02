package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
)

// RunCommand executes a shell command and returns output
func RunCommand(command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
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
	if runtime.GOOS == "darwin" && isatty.IsTerminal(os.Stdout.Fd()) {
		// Small delay to let the shell prompt reappear
		time.Sleep(150 * time.Millisecond)
		// Escape backslashes and double quotes for AppleScript
		escaped := strings.ReplaceAll(cmd, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		script := fmt.Sprintf(`tell application "System Events" to keystroke "%s"`, escaped)
		if err := exec.Command("osascript", "-e", script).Run(); err != nil {
			fmt.Printf("\nSuggested command: %s\n, err:%v", cmd, err)
		}
	} else {
		fmt.Printf("\nSuggested command: %s\n", cmd)
	}
}
