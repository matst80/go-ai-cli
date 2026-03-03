package sessions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/matst80/go-ai-cli/pkg/ollama"
)

// Session represents a saved chat session
type Session struct {
	ID        string           `json:"id"`
	Messages  []ollama.Message `json:"messages"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// getSessionsDir returns the path to the sessions directory
func getSessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".ai-cli", "sessions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create sessions directory: %w", err)
	}
	return dir, nil
}

// LoadSession loads a session by ID. If id is "last", it loads the most recently modified session.
func LoadSession(id string) (*Session, error) {
	dir, err := getSessionsDir()
	if err != nil {
		return nil, err
	}

	var filename string
	if id == "last" || id == "" {
		// Find the most recently modified file
		files, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to read sessions directory: %w", err)
		}

		var latestFile string
		var latestTime time.Time

		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".json") {
				info, err := f.Info()
				if err == nil {
					if info.ModTime().After(latestTime) {
						latestTime = info.ModTime()
						latestFile = f.Name()
					}
				}
			}
		}

		if latestFile == "" {
			return nil, fmt.Errorf("no previous sessions found")
		}
		filename = filepath.Join(dir, latestFile)
		// Extract ID from filename
		id = strings.TrimSuffix(latestFile, ".json")
	} else {
		filename = filepath.Join(dir, id+".json")
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}
	session.ID = id // Ensure ID is set

	return &session, nil
}

// SaveSession saves a session. If id is empty, it creates a new timestamp-based ID.
func SaveSession(id string, messages []ollama.Message) (string, error) {
	dir, err := getSessionsDir()
	if err != nil {
		return "", err
	}

	if id == "" || id == "last" {
		id = time.Now().Format("20060102_150405")
	}

	// Filter out the system message
	var filteredMessages []ollama.Message
	for _, msg := range messages {
		if msg.Role != "system" {
			filteredMessages = append(filteredMessages, msg)
		}
	}

	session := Session{
		ID:        id,
		Messages:  filteredMessages,
		UpdatedAt: time.Now(),
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal session: %w", err)
	}

	filename := filepath.Join(dir, id+".json")
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write session file: %w", err)
	}

	return id, nil
}

// ListSessions returns a list of all available sessions, sorted by modification time (newest first).
func ListSessions() ([]Session, error) {
	dir, err := getSessionsDir()
	if err != nil {
		return nil, err
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []Session
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".json") {
			filename := filepath.Join(dir, f.Name())
			data, err := os.ReadFile(filename)
			if err == nil {
				var session Session
				if err := json.Unmarshal(data, &session); err == nil {
					session.ID = strings.TrimSuffix(f.Name(), ".json")
					info, _ := f.Info()
					session.UpdatedAt = info.ModTime()
					sessions = append(sessions, session)
				}
			}
		}
	}

	// Sort newest first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}
