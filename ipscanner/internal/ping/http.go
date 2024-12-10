package ping

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"time"

	"github.com/sagernet/sing-box/ipscanner/internal/statute"
)

type HttpPingResult struct {
	AddrPort netip.AddrPort
	Proto    string
	Status   int
	Length   int
	RTT      time.Duration
	Err      error
}

func (h *HttpPingResult) Result() statute.IPInfo {
	return statute.IPInfo{AddrPort: h.AddrPort, RTT: h.RTT, CreatedAt: time.Now()}
}

func (h *HttpPingResult) Error() error {
	return h.Err
}

func (h *HttpPingResult) String() string {
	if h.Err != nil {
		return fmt.Sprintf("%s", h.Err)
	}

	return fmt.Sprintf("%s: protocol=%s, status=%d, length=%d, time=%d ms", h.AddrPort, h.Proto, h.Status, h.Length, h.RTT)
}

type HttpPing struct {
	Method string
	URL    string
	IP     netip.Addr

	opts statute.ScannerOptions
}

func (h *HttpPing) Ping() statute.IPingResult {
	return h.PingContext(context.Background())
}

func (h *HttpPing) PingContext(ctx context.Context) statute.IPingResult {
	u, err := url.Parse(h.URL)
	if err != nil {
		return h.errorResult(err)
	}
	orighost := u.Host

	if !h.IP.IsValid() {
		return h.errorResult(errors.New("no IP specified"))
	}

	req, err := http.NewRequestWithContext(ctx, h.Method, h.URL, nil)
	if err != nil {
		return h.errorResult(err)
	}
	ua := "httping"
	if h.opts.UserAgent != "" {
		ua = h.opts.UserAgent
	}
	req.Header.Set("User-Agent", ua)
	if h.opts.Referrer != "" {
		req.Header.Set("Referer", h.opts.Referrer)
	}
	req.Host = orighost

	addr := netip.AddrPortFrom(h.IP, h.opts.Port)
	client := h.opts.HttpClientFunc(h.opts.RawDialerFunc, h.opts.TLSDialerFunc, h.opts.QuicDialerFunc, addr.String())

	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	t0 := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return h.errorResult(err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return h.errorResult(err)
	}

	res := HttpPingResult{
		AddrPort: addr,
		Proto:    resp.Proto,
		Status:   resp.StatusCode,
		Length:   len(body),
		RTT:      time.Since(t0),
		Err:      nil,
	}

	return &res
}

func (h *HttpPing) errorResult(err error) *HttpPingResult {
	r := &HttpPingResult{}
	r.Err = err
	return r
}

func NewHttpPing(ip netip.Addr, method, url string, opts *statute.ScannerOptions) *HttpPing {
	return &HttpPing{
		IP:     ip,
		Method: method,
		URL:    url,

		opts: *opts,
	}
}

var (
	_ statute.IPing       = (*HttpPing)(nil)
	_ statute.IPingResult = (*HttpPingResult)(nil)
)
