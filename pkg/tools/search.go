package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
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
