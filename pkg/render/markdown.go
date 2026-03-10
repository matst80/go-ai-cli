package render

import (
	"strings"
)

// EscapeHTMLOutsideCodeBlocks escapes '<' to prevent markdown parser from eating unknown HTML tags
func EscapeHTMLOutsideCodeBlocks(content string) string {
	var result strings.Builder
	inCodeBlock := false
	inInlineCode := false

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
		} else if !inCodeBlock {
			line = processInlineCode(line, &inInlineCode)
		}

		result.WriteString(line)
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}
	return result.String()
}

func processInlineCode(line string, inInlineCode *bool) string {
	var processed strings.Builder
	chars := []rune(line)
	for j := 0; j < len(chars); j++ {
		if chars[j] == '`' {
			*inInlineCode = !*inInlineCode
		} else if chars[j] == '<' && !*inInlineCode {
			processed.WriteString("\\<")
			continue
		}
		processed.WriteRune(chars[j])
	}
	return processed.String()
}
