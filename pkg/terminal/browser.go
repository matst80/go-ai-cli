package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// BraveSearch performs a web search using the Brave Search API
func BraveSearch(query, country string, count, offset int) (string, error) {
	apiKey := os.Getenv("BRAVE_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("BRAVE_API_KEY environment variable is not set")
	}

	if count <= 0 {
		count = 10
	}

	u, err := url.Parse("https://api.search.brave.com/res/v1/web/search")
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("q", query)
	q.Set("result_filter", "web")
	q.Set("count", strconv.Itoa(count))
	q.Set("offset", strconv.Itoa(offset))
	if country != "" {
		q.Set("country", country)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("brave search api returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	var sb strings.Builder
	for i, res := range result.Web.Results {
		sb.WriteString(fmt.Sprintf("%d. %s\n   URL: %s\n   %s\n\n", i+1, res.Title, res.URL, res.Description))
	}

	if sb.Len() == 0 {
		return "No results found.", nil
	}

	return sb.String(), nil
}

// ChromeCDP performs a browser action using Chrome DevTools Protocol
func ChromeCDP(url, action, selector, value string) (string, error) {
	tabCtx, cancelTab, err := getCDPContext()
	if err != nil {
		return "", fmt.Errorf("failed to get CDP context: %v", err)
	}

	// For navigate, we want to leak the tab so it stays open in the browser.
	// For other actions that don't transition the page, we might want to keep it open too
	// but for now let's stick to the existing leak logic for navigate.
	if action != "navigate" && action != "click" && action != "type" {
		defer cancelTab()
	}

	// Create a timeout for the entire browser action
	timeout := 30 * time.Second
	runCtx, cancelRun := context.WithTimeout(tabCtx, timeout)
	defer cancelRun()

	var res string
	var actions []chromedp.Action

	switch action {
	case "scrape":
		if url != "" {
			actions = append(actions, chromedp.Navigate(url))
		}
		actions = append(actions,
			chromedp.WaitVisible("body", chromedp.ByQuery),
			chromedp.Evaluate(`document.body.innerText`, &res),
		)
	case "screenshot":
		if url != "" {
			actions = append(actions, chromedp.Navigate(url))
		}
		var buf []byte
		actions = append(actions,
			chromedp.WaitVisible("body", chromedp.ByQuery),
			chromedp.FullScreenshot(&buf, 100),
		)
		err = chromedp.Run(runCtx, actions...)
		if err == nil {
			filename := fmt.Sprintf("screenshot_%d.png", time.Now().Unix())
			if err := os.WriteFile(filename, buf, 0644); err != nil {
				return "", err
			}
			return fmt.Sprintf("Screenshot saved to %s", filename), nil
		}
	case "navigate":
		err = chromedp.Run(tabCtx, chromedp.Navigate(url))
		return fmt.Sprintf("Navigated to %s", url), err
	case "click":
		actions = append(actions,
			chromedp.WaitVisible(selector, chromedp.ByQuery),
			chromedp.Click(selector, chromedp.ByQuery),
		)
		res = fmt.Sprintf("Clicked %s", selector)
	case "type":
		actions = append(actions,
			chromedp.WaitVisible(selector, chromedp.ByQuery),
			chromedp.SendKeys(selector, value, chromedp.ByQuery),
		)
		res = fmt.Sprintf("Typed into %s", selector)
	case "scroll":
		actions = append(actions,
			chromedp.ScrollIntoView(selector, chromedp.ByQuery),
		)
		res = fmt.Sprintf("Scrolled to %s", selector)
	case "evaluate":
		actions = append(actions,
			chromedp.Evaluate(value, &res),
		)
	case "view_ax_tree":
		// Get simplified AX tree for the agent to understand the page structure
		actions = append(actions,
			chromedp.Evaluate(`
				(function() {
					function getRole(el) {
						if (el.getAttribute('role')) return el.getAttribute('role');
						if (el.tagName === 'BUTTON') return 'button';
						if (el.tagName === 'A') return 'link';
						if (el.tagName === 'INPUT') return el.type || 'text';
						if (el.tagName === 'SELECT') return 'combobox';
						if (el.tagName === 'TEXTAREA') return 'textbox';
						if (el.tagName === 'H1' || el.tagName === 'H2' || el.tagName === 'H3' || el.tagName === 'H4' || el.tagName === 'H5' || el.tagName === 'H6') return 'heading';
						return null;
					}
					function describe(el) {
						let role = getRole(el);
						if (!role) return null;
						let label = el.innerText || el.value || el.placeholder || el.getAttribute('aria-label') || '';
						label = label.trim().substring(0, 50);
						let selector = el.tagName.toLowerCase();
						if (el.id) selector += '#' + el.id;
						if (el.className) selector += '.' + el.className.split(/\s+/).join('.');
						return { role, label, selector };
					}
					let interactive = Array.from(document.querySelectorAll('button, a, input, select, textarea, [role], h1, h2, h3, h4, h5, h6'))
						.map(describe)
						.filter(x => x !== null);
					return JSON.stringify(interactive, null, 2);
				})()
			`, &res),
		)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}

	if err == nil {
		err = chromedp.Run(runCtx, actions...)
	}

	if err != nil {
		return "", err
	}

	if len(res) > 5000 {
		res = res[:5000] + "\n... (truncated)"
	}

	return res, nil
}

func getCDPContext() (context.Context, context.CancelFunc, error) {
	wsURL := os.Getenv("CHROME_REMOTE_URL")
	isRemote := wsURL != ""

	if isRemote {
		// Construct ws:// URL if only host/port provided.
		if strings.HasPrefix(wsURL, "http://") {
			wsURL = "ws://" + wsURL[7:]
		} else if strings.HasPrefix(wsURL, "https://") {
			wsURL = "wss://" + wsURL[8:]
		}
		if !strings.HasPrefix(wsURL, "ws://") && !strings.HasPrefix(wsURL, "wss://") {
			if !strings.Contains(wsURL, ":") {
				wsURL = "127.0.0.1:" + wsURL
			}
			wsURL = "ws://" + wsURL
		}
	} else {
		wsURL = "ws://127.0.0.1:9222"
	}

	// 1. Setup allocator
	allocatorCtx, _ := chromedp.NewRemoteAllocator(context.Background(), wsURL)

	// 2. If local, try to connect/start Chrome
	if !isRemote {
		// Quick check if already running
		testCtx, cancel := chromedp.NewContext(allocatorCtx)
		testCtx, timeout := context.WithTimeout(testCtx, 1*time.Second)
		err := chromedp.Run(testCtx, chromedp.Navigate("about:blank"))
		timeout()
		cancel()

		if err != nil {
			// Start Chrome if not running
			userDataDir := os.Getenv("HOME") + "/.go-ai-cli/chrome-profile"
			_ = os.MkdirAll(userDataDir, 0755)

			cmd := exec.Command("open", "-na", "Google Chrome", "--args",
				"--remote-debugging-port=9222",
				"--user-data-dir="+userDataDir,
				"--no-first-run",
				"--no-default-browser-check",
			)
			if err := cmd.Start(); err != nil {
				return nil, nil, fmt.Errorf("failed to start Chrome: %v", err)
			}

			// Wait for it to be ready
			for i := 0; i < 20; i++ {
				time.Sleep(500 * time.Millisecond)
				allocatorCtx, _ = chromedp.NewRemoteAllocator(context.Background(), wsURL)
				testCtx, cancel = chromedp.NewContext(allocatorCtx)
				testCtx, timeout = context.WithTimeout(testCtx, 1*time.Second)
				if err := chromedp.Run(testCtx, chromedp.Navigate("about:blank")); err == nil {
					timeout()
					cancel()
					break
				}
				timeout()
				cancel()
				if i == 19 {
					return nil, nil, fmt.Errorf("chrome failed to start on port 9222")
				}
			}
		}
	}

	// 3. Try to reuse an existing page tab instead of creating a new one
	targets, err := chromedp.Targets(allocatorCtx)
	if err == nil {
		for _, t := range targets {
			if t.Type == "page" && !strings.Contains(t.URL, "devtools://") {
				// Found an existing tab! Attach to it.
				ctx, cancelTab := chromedp.NewContext(allocatorCtx, chromedp.WithTargetID(t.TargetID))
				return ctx, cancelTab, nil
			}
		}
	}

	// 4. Create a new tab if no existing page found
	ctx, cancelTab := chromedp.NewContext(allocatorCtx)
	return ctx, cancelTab, nil
}
