package ghclient

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var defaultHTTPClient = &http.Client{
	Timeout: 20 * time.Second,
}

func Fetch(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	token := os.Getenv("GITHUB_TOKEN")
	token = strings.TrimSpace(token)
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	res, err := defaultHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	UpdateRateLimitFromHeaders(res.Header)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP request failed with status code %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
