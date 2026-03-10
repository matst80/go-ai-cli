package tools

import (
	"context"
	"os/exec"
	"strings"
	"time"
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
