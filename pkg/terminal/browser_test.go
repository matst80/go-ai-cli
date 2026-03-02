package terminal

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestChromeCDP_Scrape(t *testing.T) {
	// Create a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "<html><body><h1>Hello, World!</h1><p>Test Content</p></body></html>")
	}))
	defer ts.Close()

	// Test scrape
	output, err := ChromeCDP(ts.URL, "scrape")
	if err != nil {
		t.Fatalf("ChromeCDP scrape failed: %v", err)
	}

	if !strings.Contains(output, "Hello, World!") {
		t.Errorf("Expected output to contain 'Hello, World!', got: %s", output)
	}
	if !strings.Contains(output, "Test Content") {
		t.Errorf("Expected output to contain 'Test Content', got: %s", output)
	}
}

func TestChromeCDP_Screenshot(t *testing.T) {
	// Create a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "<html><body><h1>Screenshot Test</h1></body></html>")
	}))
	defer ts.Close()

	// Test screenshot
	output, err := ChromeCDP(ts.URL, "screenshot")
	if err != nil {
		t.Fatalf("ChromeCDP screenshot failed: %v", err)
	}

	if !strings.HasPrefix(output, "Screenshot saved to") {
		t.Errorf("Expected output to start with 'Screenshot saved to', got: %s", output)
	}

	// Clean up screenshot file
	parts := strings.Split(output, " ")
	filename := parts[len(parts)-1]
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Errorf("Screenshot file %s was not created", filename)
	} else {
		os.Remove(filename)
	}
}

func TestChromeCDP_Navigate(t *testing.T) {
	// Create a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "<html><body><h1>Navigate Test</h1></body></html>")
	}))
	defer ts.Close()

	// Test navigate (now persists tab)
	output, err := ChromeCDP(ts.URL, "navigate")
	if err != nil {
		t.Fatalf("ChromeCDP navigate failed: %v", err)
	}

	if !strings.Contains(output, "Navigated to") {
		t.Errorf("Expected output to contain 'Navigated to', got: %s", output)
	}
}

func TestChromeCDP_Error(t *testing.T) {
	_, err := ChromeCDP("http://invalid-url-that-does-not-exist.local", "scrape")
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}

	_, err = ChromeCDP("http://google.com", "invalid_action")
	if err == nil {
		t.Error("Expected error for invalid action, got nil")
	}
}
