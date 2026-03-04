package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type ModelOptions struct {
	Temperature   float64 `json:"temperature,omitempty"`
	NumCtx        int     `json:"num_ctx,omitempty"`
	RepeatPenalty float64 `json:"repeat_penalty,omitempty"`
	TopP          float64 `json:"top_p,omitempty"`
	TopK          int     `json:"top_k,omitempty"`
	NumPredict    int     `json:"num_predict,omitempty"`
}

// Config represents the application configuration
type Config struct {
	SystemPrompt string       `json:"system_prompt,omitempty"`
	Memory       []string     `json:"memory,omitempty"`
	Yolo         bool         `json:"yolo,omitempty"`
	Style        string       `json:"style,omitempty"`
	URL          string       `json:"url,omitempty"`
	Model        string       `json:"model,omitempty"`
	ModelOptions ModelOptions `json:"model_options,omitempty"`
	Thinking     bool         `json:"thinking,omitempty"`
	CDP          string       `json:"-"` // Not saved to file
	Resume       string       `json:"-"` // Not saved to file
	SaveSession  bool         `json:"-"` // Not saved to file
}

// GetConfigPath returns the default configuration path (~/.ai-cli/config)
func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ai-cli", "config"), nil
}

// Load reads the configuration from the config file, environment variables, and flags
func Load() (*Config, error) {
	cfg := &Config{}
	path, err := GetConfigPath()
	if err == nil {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, cfg)
		}
	}

	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = fmt.Sprintf(`You are a terminal expert for %s.
Please respond with markdown.

To save a code block to a specific file path, use the format:
%[2]slanguage:filename.ext
content
%[2]s
`, runtime.GOOS, "```")
	}

	// Environment variable overrides
	if envURL := os.Getenv("OLLAMA_URL"); envURL != "" {
		cfg.URL = envURL
	}
	if envModel := os.Getenv("OLLAMA_MODEL"); envModel != "" {
		cfg.Model = envModel
	}
	if envStyle := os.Getenv("AI_STYLE"); envStyle != "" {
		cfg.Style = envStyle
	}
	if os.Getenv("AI_YOLO") == "true" {
		cfg.Yolo = true
	}
	if os.Getenv("AI_THINKING") == "true" {
		cfg.Thinking = true
	}
	if envCDP := os.Getenv("CHROME_REMOTE_URL"); envCDP != "" {
		cfg.CDP = envCDP
	}

	// Flag definition and parsing
	// Note: these will override file and env values if provided
	var cdpFlag *string
	var styleFlag *string
	var yoloFlag *bool
	var urlFlag *string
	var modelFlag *string
	var thinkingFlag *bool
	var resumeFlag *string
	var saveFlag *bool

	if flag.Lookup("cdp") == nil {
		cdpFlag = flag.String("cdp", cfg.CDP, "Remote CDP URL or port (e.g. 9222 or localhost:9222)")
		styleFlag = flag.String("style", cfg.Style, "Output style (dark, light, or auto)")
		yoloFlag = flag.Bool("yolo", cfg.Yolo, "Run all commands without confirmation")
		thinkingFlag = flag.Bool("thinking", cfg.Thinking, "Enable thinking/reasoning for Ollama")
		urlFlag = flag.String("url", cfg.URL, "Ollama API URL")
		modelFlag = flag.String("model", cfg.Model, "Ollama model name")
		resumeFlag = flag.String("resume", "", "Resume a session by ID or 'last'")
		saveFlag = flag.Bool("save", false, "Save the session")
	} else {
		// Replace values if already defined (for tests or multiple calls)
		cf := flag.Lookup("cdp")
		_ = cf.Value.Set(cfg.CDP)
		sf := flag.Lookup("style")
		_ = sf.Value.Set(cfg.Style)
		yf := flag.Lookup("yolo")
		_ = yf.Value.Set(fmt.Sprintf("%v", cfg.Yolo))
		tf := flag.Lookup("thinking")
		_ = tf.Value.Set(fmt.Sprintf("%v", cfg.Thinking))
		uf := flag.Lookup("url")
		_ = uf.Value.Set(cfg.URL)
		mf := flag.Lookup("model")
		_ = mf.Value.Set(cfg.Model)
		rf := flag.Lookup("resume")
		_ = rf.Value.Set(cfg.Resume)
		saf := flag.Lookup("save")
		_ = saf.Value.Set(fmt.Sprintf("%v", cfg.SaveSession))

		// Create local pointers to match original logic
		cdpStr := cf.Value.String()
		cdpFlag = &cdpStr
		styleStr := sf.Value.String()
		styleFlag = &styleStr
		yoloVal := yf.Value.(flag.Getter).Get().(bool)
		yoloFlag = &yoloVal
		thinkVal := tf.Value.(flag.Getter).Get().(bool)
		thinkingFlag = &thinkVal
		urlStr := uf.Value.String()
		urlFlag = &urlStr
		modelStr := mf.Value.String()
		modelFlag = &modelStr
		resumeStr := rf.Value.String()
		resumeFlag = &resumeStr
		saveVal := saf.Value.(flag.Getter).Get().(bool)
		saveFlag = &saveVal
	}

	if !flag.Parsed() {
		flag.Parse()
	}

	// Handle flag overrides
	cfg.CDP = *cdpFlag
	cfg.Style = *styleFlag
	cfg.Yolo = *yoloFlag
	cfg.Thinking = *thinkingFlag
	cfg.URL = *urlFlag
	cfg.Model = *modelFlag
	cfg.Resume = *resumeFlag
	cfg.SaveSession = *saveFlag

	// Hardcoded defaults
	if cfg.URL == "" {
		cfg.URL = "http://localhost:11434/api/chat"
	}
	if cfg.Model == "" {
		cfg.Model = "ministral-3:latest"
	}
	if cfg.Style == "" {
		cfg.Style = "auto"
	}

	// Default num_ctx if not specified
	if cfg.ModelOptions.NumCtx == 0 {
		cfg.ModelOptions.NumCtx = 16384 // Default as set in main.go
	}

	// Sync back to Env for sub-packages
	if cfg.Yolo {
		os.Setenv("AI_YOLO", "true")
	}
	if cfg.Thinking {
		os.Setenv("AI_THINKING", "true")
	}
	if cfg.Style != "" {
		os.Setenv("AI_STYLE", cfg.Style)
	}
	if cfg.CDP != "" {
		os.Setenv("CHROME_REMOTE_URL", cfg.CDP)
	}

	return cfg, nil
}

// Save writes the configuration to the config file
func (c *Config) Save() error {
	path, err := GetConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
