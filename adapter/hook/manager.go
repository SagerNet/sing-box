package hook

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/sagernet/sing-box/option"
)

type Manager struct {
	hook         *option.HookOptions
	httpExecutor *http.Client
}

func NewManager(hook *option.HookOptions) *Manager {
	return &Manager{
		hook: hook,
		httpExecutor: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			CheckRedirect: redirectChecker(false),
		},
	}
}
func (m *Manager) PreStart() error {
	if m.hook == nil {
		return nil
	}
	return m.execute(m.hook.PreStart)
}
func (m *Manager) PostStart() error {
	if m.hook == nil {
		return nil
	}
	return m.execute(m.hook.PostStart)
}
func (m *Manager) PreStop() error {
	if m.hook == nil {
		return nil
	}
	return m.execute(m.hook.PreStop)
}
func (m *Manager) PostStop() error {
	if m.hook == nil {
		return nil
	}
	return m.execute(m.hook.PostStop)
}
func redirectChecker(followNonLocalRedirects bool) func(*http.Request, []*http.Request) error {
	if followNonLocalRedirects {
		return nil
	}
	return func(req *http.Request, via []*http.Request) error {
		if req.URL.Hostname() != via[0].URL.Hostname() {
			return http.ErrUseLastResponse
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
}
func (m *Manager) execute(execution option.Hook) error {
	for _, httpExecution := range execution.HTTP {
		if err := m.executeHTTP(&httpExecution); err != nil && execution.HandleError {
			return err
		}
	}
	return nil
}
func (m *Manager) executeHTTP(httpExecution *option.HTTPExecution) error {
	if httpExecution == nil {
		return nil
	}
	req, err := m.buildRequest(httpExecution)
	if err != nil {
		return err
	}
	resp, err := m.httpExecutor.Do(req)
	discardHTTPRespBody(resp)
	if isHTTPResponseError(err) {
		req := req.Clone(context.Background())
		req.URL.Scheme = "http"
		req.Header.Del("Authorization")
		resp, httpErr := m.httpExecutor.Do(req)
		if httpErr == nil {
			err = nil
		}
		discardHTTPRespBody(resp)
	}
	return err
}
func isHTTPResponseError(err error) bool {
	if err == nil {
		return false
	}
	urlErr := &url.Error{}
	if !errors.As(err, &urlErr) {
		return false
	}
	return strings.Contains(urlErr.Err.Error(), "server gave HTTP response to HTTPS client")
}

const (
	maxRespBodyLength = 10 * 1 << 10
)

func discardHTTPRespBody(resp *http.Response) {
	if resp == nil {
		return
	}
	defer resp.Body.Close()
	if resp.ContentLength <= maxRespBodyLength {
		io.Copy(io.Discard, &io.LimitedReader{R: resp.Body, N: maxRespBodyLength})
	}
}
func (m *Manager) buildRequest(httpExecution *option.HTTPExecution) (*http.Request, error) {
	u, err := url.Parse(httpExecution.URL)
	if err != nil {
		return nil, err
	}
	headers := buildHeader(httpExecution.Headers)
	return newProbeRequest(u, headers)
}
func newProbeRequest(url *url.URL, headers http.Header) (*http.Request, error) {
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, err
	}
	if headers == nil {
		headers = http.Header{}
	}
	if _, ok := headers["User-Agent"]; !ok {
		headers.Set("User-Agent", "TODO://")
	}
	if _, ok := headers["Accept"]; !ok {
		headers.Set("Accept", "*/*")
	} else if headers.Get("Accept") == "" {
		headers.Del("Accept")
	}
	req.Header = headers
	req.Host = headers.Get("Host")
	return req, nil
}
func buildHeader(headerList []option.Header) http.Header {
	headers := make(http.Header)
	for _, header := range headerList {
		headers.Add(header.Name, header.Value)
	}
	return headers
}
