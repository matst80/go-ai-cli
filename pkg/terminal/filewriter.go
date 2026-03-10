package terminal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StreamHandler intercepts streamed content looking for code blocks.
// Formats supported:
//
//	```lang:path/to/file.ext or :::lang:path/to/file.ext
//	```lang or :::lang
//
// Every code block is saved. If a filename is provided (via : suffix), it's saved there.
// Otherwise, it's saved to a unique temporary file in ~/.ai-cli/tmp.
// If the language is 'bash' or 'sh', it is also suggested as a command.
type StreamHandler struct {
	fs               FileService
	emit             func(text string)                           // forward non-block text
	fileSaved        func(filename, content string, isTemp bool) // notify on file save
	commandSuggested func(command string)                        // notify on command suggestion
	inBlock          bool
	isCommand        bool
	lang             string
	filename         string
	buf              strings.Builder
	lineBuffer       string // partial line accumulator
}

func NewStreamHandler(fs FileService, emit func(string), fileSaved func(string, string, bool), commandSuggested func(string)) *StreamHandler {
	if fs == nil {
		fs = NewDefaultFileService()
	}
	return &StreamHandler{fs: fs, emit: emit, fileSaved: fileSaved, commandSuggested: commandSuggested}
}

// Feed processes a chunk of streamed text (may be partial lines).
func (sh *StreamHandler) Feed(chunk string) {
	// Combine with any leftover from previous chunk
	data := sh.lineBuffer + chunk
	sh.lineBuffer = ""

	// Process complete lines; keep any trailing partial line
	for {
		idx := strings.Index(data, "\n")
		if idx == -1 {
			// No newline: might be a partial marker, buffer it
			sh.lineBuffer = data
			return
		}
		line := data[:idx]
		data = data[idx+1:]
		sh.processLine(line)
	}
}

// Flush should be called when the stream ends to emit any remaining buffered text.
func (sh *StreamHandler) Flush() {
	if sh.lineBuffer != "" {
		if sh.inBlock {
			sh.emit(sh.lineBuffer + "\n")
			sh.buf.WriteString(sh.lineBuffer)
			sh.completeBlock()
		} else {
			sh.emit(sh.lineBuffer)
		}
		sh.lineBuffer = ""
	}
	if sh.inBlock {
		sh.completeBlock()
	}
}

func (sh *StreamHandler) processLine(line string) {
	trimmed := strings.TrimSpace(line)

	// Start markers: ```lang or :::lang (optionally :filename)
	if !sh.inBlock && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, ":::")) {
		prefix := "```"
		if strings.HasPrefix(trimmed, ":::") {
			prefix = ":::"
		}

		tag := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		sh.inBlock = true
		sh.buf.Reset()
		sh.lang = ""
		sh.filename = ""
		sh.isCommand = false

		if tag != "" {
			if idx := strings.Index(tag, ":"); idx != -1 {
				sh.lang = strings.TrimSpace(tag[:idx])
				sh.filename = strings.TrimSpace(tag[idx+1:])
			} else if strings.HasPrefix(tag, "file.") {
				// Legacy/alternate file support
				sh.lang = "file"
				sh.filename = strings.TrimPrefix(tag, "file.")
			} else {
				sh.lang = tag
			}
		}

		if sh.lang == "bash" || sh.lang == "sh" {
			sh.isCommand = true
			//sh.emit("\n**Suggested Command:**\n") // Removed to follow "only last one" and avoid buffering headers
		} else if sh.filename != "" {
			sh.emit(fmt.Sprintf("\n**Writing:** `%s`\n", sh.filename))
		}
		sh.emit("```" + sh.lang + "\n")
		return
	}

	// End marker: ::: or ```
	if sh.inBlock && (trimmed == ":::" || trimmed == "```") {
		sh.completeBlock()
		return
	}

	if sh.inBlock {
		if sh.buf.Len() > 0 {
			sh.buf.WriteString("\n")
		}
		sh.buf.WriteString(line)
		sh.emit(line + "\n")
		return
	}

	// Normal text — pass through with the newline
	sh.emit(line + "\n")
}

func (sh *StreamHandler) completeBlock() {
	if sh.inBlock {
		sh.emit("```\n")
		sh.inBlock = false
	}

	content := sh.buf.String()
	isCmd := sh.isCommand
	filename := sh.filename
	lang := sh.lang

	sh.isCommand = false
	sh.filename = ""
	sh.lang = ""
	sh.buf.Reset()

	if content == "" {
		return
	}

	isTemp := false
	if filename == "" {
		// Don't save one-liners to temp files
		if !strings.Contains(content, "\n") {
			if isCmd && sh.commandSuggested != nil {
				sh.commandSuggested(content)
			}
			return
		}
		filename = sh.generateTempFile(lang)
		isTemp = true
	}

	dir := filepath.Dir(filename)
	if err := sh.fs.MkdirAll(dir, 0755); err != nil {
		sh.emit(fmt.Sprintf("\n❌ **Error creating dir:** %v\n", err))
		return
	}
	if err := sh.fs.WriteFile(filename, []byte(content), 0644); err != nil {
		sh.emit(fmt.Sprintf("\n❌ **Error writing file:** %v\n", err))
		return
	}

	if sh.fileSaved != nil {
		sh.fileSaved(filename, content, isTemp)
	}

	if isCmd && sh.commandSuggested != nil {
		sh.commandSuggested(content)
	}
}

func (sh *StreamHandler) generateTempFile(lang string) string {
	home, _ := os.UserHomeDir()
	tmpDir := filepath.Join(home, ".ai-cli", "tmp")

	ext := lang
	if ext == "" {
		ext = "txt"
	}
	// Sanitize extension
	ext = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, ext)

	timestamp := time.Now().Format("20060102_150405")
	nano := time.Now().UnixNano() % 1e6
	filename := fmt.Sprintf("snippet_%s_%d.%s", timestamp, nano, ext)
	return filepath.Join(tmpDir, filename)
}

// InBlock returns whether we're currently inside a block.
func (sh *StreamHandler) InBlock() bool {
	return sh.inBlock
}

// CurrentFilename returns the filename being written, if in a file block.
func (sh *StreamHandler) CurrentFilename() string {
	return sh.filename
}
