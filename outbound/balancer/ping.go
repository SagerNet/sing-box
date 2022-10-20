package balancer

import (
	"context"
	"net/http"
	"time"

	"net"

	"github.com/sagernet/sing-box/log"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type pingClient struct {
	destination string
	httpClient  *http.Client
}

func newPingClient(detour N.Dialer, destination string, timeout time.Duration) *pingClient {
	return &pingClient{
		destination: destination,
		httpClient:  newHTTPClient(detour, timeout),
	}
}

func newDirectPingClient(destination string, timeout time.Duration) *pingClient {
	return &pingClient{
		destination: destination,
		httpClient:  &http.Client{Timeout: timeout},
	}
}

func newHTTPClient(detour N.Dialer, timeout time.Duration) *http.Client {
	tr := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			ctx = log.ContextWithOverrideLevel(ctx, log.LevelDebug)
			return detour.DialContext(ctx, network, M.ParseSocksaddr(addr))
		},
	}
	return &http.Client{
		Transport: tr,
		Timeout:   timeout,
		// don't follow redirect
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// MeasureDelay returns the delay time of the request to dest
func (s *pingClient) MeasureDelay() (time.Duration, error) {
	if s.httpClient == nil {
		panic("pingClient no initialized")
	}
	req, err := http.NewRequest(http.MethodHead, s.destination, nil)
	if err != nil {
		return rttFailed, err
	}
	start := time.Now()
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return rttFailed, err
	}
	// don't wait for body
	resp.Body.Close()
	return time.Since(start), nil
}
