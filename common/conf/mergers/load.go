package mergers

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// loadToBytes loads one arg to []byte, maybe an remote url, or local file path
func loadToBytes(arg string) (out []byte, err error) {
	switch {
	case strings.HasPrefix(arg, "http://"), strings.HasPrefix(arg, "https://"):
		out, err = fetchHTTPContent(arg)
	default:
		out, err = ioutil.ReadFile(arg)
	}
	if err != nil {
		return
	}
	return
}

// fetchHTTPContent dials https for remote content
func fetchHTTPContent(target string) ([]byte, error) {
	parsedTarget, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %s", target)
	}

	if s := strings.ToLower(parsedTarget.Scheme); s != "http" && s != "https" {
		return nil, fmt.Errorf("invalid scheme: %s", parsedTarget.Scheme)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(&http.Request{
		Method: "GET",
		URL:    parsedTarget,
		Close:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to dial to %s", target)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New("failed to read HTTP response")
	}

	return content, nil
}
