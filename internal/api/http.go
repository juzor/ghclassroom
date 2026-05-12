package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func doRequest(token, method, url string, out interface{}) error {
	_, err := doRequestWithNext(token, method, url, out)
	return err
}

// doRequestWithNext performs one HTTP request and returns the next-page URL
// from the Link header (empty string if there is no next page).
func doRequestWithNext(token, method, url string, out interface{}) (nextURL string, err error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.Header.Get("X-RateLimit-Remaining") == "0" {
		return "", &RateLimitError{ResetAt: parseResetTime(resp.Header.Get("X-RateLimit-Reset"))}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return "", err
	}
	return parseLinkNext(resp.Header.Get("Link")), nil
}

// parseLinkNext extracts the URL with rel="next" from a GitHub Link header.
func parseLinkNext(link string) string {
	for _, part := range strings.Split(link, ",") {
		part = strings.TrimSpace(part)
		sections := strings.Split(part, ";")
		if len(sections) != 2 {
			continue
		}
		if strings.TrimSpace(sections[1]) == `rel="next"` {
			url := strings.TrimSpace(sections[0])
			return strings.Trim(url, "<>")
		}
	}
	return ""
}

func parseResetTime(unixStr string) time.Time {
	ts, err := strconv.ParseInt(unixStr, 10, 64)
	if err != nil || ts == 0 {
		return time.Now().Add(60 * time.Second)
	}
	return time.Unix(ts, 0)
}
