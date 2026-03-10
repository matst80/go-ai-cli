package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

)

// ChromeCDP performs a browser action using Chrome DevTools Protocol
func ChromeCDP(url, action, selector, value string) (string, []string, error) {
	tabCtx, err := getCDPContext()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get CDP context: %v", err)
	}

	var res string
	var images []string
	var actions []chromedp.Action

	// Normalize URL
	url = NormalizeURL(url)

	// Avoid redundant navigation
	if url != "" {
		var currentURL string
		_ = chromedp.Run(tabCtx, chromedp.Location(&currentURL))
		if !strings.HasPrefix(currentURL, url) && !strings.HasPrefix(url, currentURL) {
			actions = append(actions, chromedp.Navigate(url))
		}
	}

	switch action {
	case "scrape":
		actions = append(actions,
			chromedp.WaitVisible("body", chromedp.ByQuery),
			chromedp.Evaluate(`document.body.innerText`, &res),
		)
	case "screenshot":
		var buf []byte
		actions = append(actions,
			chromedp.WaitVisible("body", chromedp.ByQuery),
			chromedp.FullScreenshot(&buf, 100),
		)
		err = chromedp.Run(tabCtx, actions...)
		if err == nil {
			filename := fmt.Sprintf("screenshot_%d.png", time.Now().Unix())
			if err := os.WriteFile(filename, buf, 0644); err != nil {
				return "", nil, err
			}
			imgBase64 := base64.StdEncoding.EncodeToString(buf)
			images = append(images, imgBase64)
			return fmt.Sprintf("Screenshot saved to %s", filename), images, nil
		}
	case "navigate":
		url = NormalizeURL(url)
		err = chromedp.Run(tabCtx, chromedp.Navigate(url))
		return fmt.Sprintf("Navigated to %s", url), nil, err
	case "click":
		if selector == "" {
			return "", nil, fmt.Errorf("selector is required for click action")
		}
		actions = append(actions,
			chromedp.WaitVisible(selector, chromedp.ByQuery),
			chromedp.Click(selector, chromedp.ByQuery),
		)
		res = fmt.Sprintf("Clicked %s", selector)
	case "type":
		if selector == "" {
			return "", nil, fmt.Errorf("selector is required for type action")
		}
		actions = append(actions,
			chromedp.WaitVisible(selector, chromedp.ByQuery),
			chromedp.SendKeys(selector, value, chromedp.ByQuery),
		)
		res = fmt.Sprintf("Typed into %s", selector)
	case "scroll":
		if selector == "" || selector == "window" || selector == "body" {
			v := strings.ToLower(value)
			switch v {
			case "down", "pagedown":
				actions = append(actions, chromedp.Evaluate(`window.scrollBy(0, window.innerHeight)`, nil))
				res = "Scrolled down one page"
			case "up", "pageup":
				actions = append(actions, chromedp.Evaluate(`window.scrollBy(0, -window.innerHeight)`, nil))
				res = "Scrolled up one page"
			case "top":
				actions = append(actions, chromedp.Evaluate(`window.scrollTo(0, 0)`, nil))
				res = "Scrolled to top"
			case "bottom":
				actions = append(actions, chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight)`, nil))
				res = "Scrolled to bottom"
			default:
				// If value is a number, scroll by that amount
				if pixels, err := strconv.Atoi(value); err == nil {
					actions = append(actions, chromedp.Evaluate(fmt.Sprintf(`window.scrollBy(0, %d)`, pixels), nil))
					res = fmt.Sprintf("Scrolled by %d pixels", pixels)
				} else {
					if selector == "" {
						selector = "body"
					}
					actions = append(actions, chromedp.ScrollIntoView(selector, chromedp.ByQuery))
					res = fmt.Sprintf("Scrolled to %s, has value: %s", selector, value)
				}
			}
		} else {
			actions = append(actions, chromedp.ScrollIntoView(selector, chromedp.ByQuery))
			res = fmt.Sprintf("Scrolled to %s", selector)
		}
	case "evaluate":
		actions = append(actions,
			chromedp.Evaluate(value, &res),
		)
	case "view_ax_tree":
		// Get simplified AX tree as a Markdown table
		filter := value // use 'value' as a filter if provided
		actions = append(actions,
			chromedp.Evaluate(fmt.Sprintf(`
				(function() {
					const filterText = %q.toLowerCase();
					function getRole(el) {
						if (el.getAttribute('role')) return el.getAttribute('role');
						const tag = el.tagName;
						if (tag === 'BUTTON') return 'button';
						if (tag === 'A') return 'link';
						if (tag === 'INPUT') return el.type || 'text';
						if (tag === 'SELECT') return 'combobox';
						if (tag === 'TEXTAREA') return 'textbox';
						if (tag.startsWith('H') && tag.length === 2 && tag[1] >= '1' && tag[1] <= '6') return 'heading';
						return null;
					}
					function describe(el) {
						if (el.getAttribute('aria-hidden') === 'true') return null;
						const style = window.getComputedStyle(el);
						if (style.display === 'none' || style.visibility === 'hidden' || style.opacity === '0') return null;
						if (!!!(el.offsetWidth || el.offsetHeight || el.getClientRects().length)) return null;
						
						const rect = el.getBoundingClientRect();
						const inView = rect.bottom > 0 && rect.right > 0 && 
						               rect.left < (window.innerWidth || document.documentElement.clientWidth) && 
						               rect.top < (window.innerHeight || document.documentElement.clientHeight);

						let role = getRole(el);
						if (!role) return null;
						let label = (el.innerText || el.value || el.placeholder || el.getAttribute('aria-label') || '').trim().replace(/\n/g, ' ');
						if (el.tagName === 'INPUT' && !label) label = el.name || '';
						if (!label) return null;
						label = label.substring(0, 50).replace(/\|/g, '\\|');
						let selector = el.tagName.toLowerCase();
						if (el.id) selector += '#' + el.id;
						if (el.className && typeof el.className === 'string') {
							const cls = el.className.split(/\s+/).filter(c => c && !c.includes(':')).join('.');
							if (cls) selector += '.' + cls;
						}
						selector = selector.replace(/\|/g, '\\|');
						if (filterText && !role.toLowerCase().includes(filterText) && !label.toLowerCase().includes(filterText) && !selector.toLowerCase().includes(filterText)) {
							return null;
						}
						return { role, label, selector, inView };
					}
					let elements = Array.from(document.querySelectorAll('button, a, input, select, textarea, [role], h1, h2, h3, h4, h5, h6'));
					let interactive = elements.map(describe).filter(x => x !== null);
					
					if (interactive.length === 0) return "No elements found" + (filterText ? " matching '" + filterText + "'" : "");
					
					let toShow = interactive;
					let filteredByView = false;
					if (interactive.length > 50) {
						const inViewElements = interactive.filter(x => x.inView);
						if (inViewElements.length > 0) {
							toShow = inViewElements;
							filteredByView = true;
						}
					}

					// Group by role
					const groups = {};
					const items = toShow.slice(0, 50);
					items.forEach(item => {
						if (!groups[item.role]) groups[item.role] = [];
						groups[item.role].push(item);
					});

					let res = "";
					if (filteredByView) {
						res += "*Showing elements in viewport (" + toShow.length + " in view out of " + interactive.length + " total results).*\n\n";
					}

					for (const role in groups) {
						res += "### " + role.toUpperCase() + "S\n";
						res += "| Selector | Label |\n|:---|:---|\n";
						groups[role].forEach(item => {
							res += "| " + item.selector + " | " + item.label + " |\n";
						});
						res += "\n";
					}

					if (toShow.length > 50) {
						res += "*Showing 50 of " + toShow.length + " " + (filteredByView ? "in-view " : "") + "elements. Use 'value' to filter.*";
					} else if (!filteredByView && interactive.length > 50) {
						res += "*Showing 50 of " + interactive.length + " elements. Use 'value' to filter.*";
					}
					return res;
				})()
			`, filter), &res),
		)
	default:
		return "", nil, fmt.Errorf("unknown action: %s", action)
	}

	if err == nil {
		// Create a separate timeout context specifically for EXECUTION, not tied to the tab lifecycle.
		// By wrapping the actions in ActionFunc, if the context is cancelled, chromedp aborts the run
		// but since tabCtx (the target) isn't the one cancelled, the tab survives.
		runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err = chromedp.Run(tabCtx, chromedp.ActionFunc(func(c context.Context) error {
			// Proxy the execution to respecting the runCtx, while operating on 'c' (the tab context)
			errCh := make(chan error, 1)
			go func() {
				errCh <- chromedp.Run(c, actions...)
			}()
			select {
			case <-runCtx.Done():
				return runCtx.Err()
			case err := <-errCh:
				return err
			}
		}))
	}

	if err != nil {
		return "", nil, err
	}

	if len(res) > 5000 {
		res = res[:5000] + "\n... (truncated)"
	}

	return res, images, nil
}

var globalCDPCtx context.Context
var globalCDPCancel context.CancelFunc

func getCDPContext() (context.Context, error) {
	if globalCDPCtx != nil {
		return globalCDPCtx, nil
	}

	wsURL := os.Getenv("CHROME_REMOTE_URL")
	isRemote := wsURL != ""

	if isRemote {
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

	allocatorCtx, _ := chromedp.NewRemoteAllocator(context.Background(), wsURL)

	if !isRemote {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:9222", 500*time.Millisecond)
		if err == nil {
			conn.Close()
		} else {
			userDataDir := os.Getenv("HOME") + "/.go-ai-cli/chrome-profile"
			_ = os.MkdirAll(userDataDir, 0755)

			cmd := exec.Command("open", "-na", "Google Chrome", "--args",
				"--remote-debugging-port=9222",
				"--user-data-dir="+userDataDir,
				"--no-first-run",
				"--no-default-browser-check",
			)
			if err := cmd.Start(); err != nil {
				return nil, fmt.Errorf("failed to start Chrome: %v", err)
			}

			// Wait for it to be ready
			for i := 0; i < 20; i++ {
				time.Sleep(500 * time.Millisecond)
				conn, err := net.DialTimeout("tcp", "127.0.0.1:9222", 500*time.Millisecond)
				if err == nil {
					conn.Close()
					break
				}
				if i == 19 {
					return nil, fmt.Errorf("chrome failed to start on port 9222")
				}
			}
		}
	}

	// Find an existing target
	targets, err := chromedp.Targets(allocatorCtx)
	if err == nil {
		for _, t := range targets {
			if t.Type == "page" && !strings.Contains(t.URL, "devtools://") && !strings.HasPrefix(t.URL, "chrome-extension://") {
				globalCDPCtx, globalCDPCancel = chromedp.NewContext(allocatorCtx, chromedp.WithTargetID(t.TargetID))
				return globalCDPCtx, nil
			}
		}
	}

	// Create a new target
	globalCDPCtx, globalCDPCancel = chromedp.NewContext(allocatorCtx)
	// Initialize it so the tab actually opens immediately
	_ = chromedp.Run(globalCDPCtx)

	return globalCDPCtx, nil
}

// NormalizeURL ensures the URL has a scheme, defaulting to https:// if missing
func NormalizeURL(u string) string {
	if u == "" {
		return ""
	}
	if !strings.Contains(u, "://") {
		return "https://" + u
	}
	return u
}
