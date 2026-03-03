package config_test

import (
	"os"
	"testing"

	"github.com/matst80/go-ai-cli/pkg/config"
)

func TestConfigLoadSave(t *testing.T) {
	// Mock HOME to use a temp directory
	tempHome, err := os.MkdirTemp("", "ai-cli-test-home-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempHome)

	os.Setenv("HOME", tempHome)

	cfg := &config.Config{
		SystemPrompt: "Test Prompt",
		Memory:       []string{"Test Memory"},
		Yolo:         true,
		Style:        "dark",
		URL:          "http://localhost:11434/api/chat",
		Model:        "test-model",
		ModelOptions: map[string]interface{}{
			"temperature": 0.7,
		},
	}

	err = cfg.Save()
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loaded.SystemPrompt != cfg.SystemPrompt {
		t.Errorf("Expected SystemPrompt %s, got %s", cfg.SystemPrompt, loaded.SystemPrompt)
	}
	if loaded.Yolo != cfg.Yolo {
		t.Errorf("Expected Yolo %v, got %v", cfg.Yolo, loaded.Yolo)
	}
	if loaded.Model != cfg.Model {
		t.Errorf("Expected Model %s, got %s", cfg.Model, loaded.Model)
	}
	if v, ok := loaded.ModelOptions["temperature"].(float64); !ok || v != 0.7 {
		t.Errorf("Expected ModelOption temperature 0.7, got %v", loaded.ModelOptions["temperature"])
	}
}
