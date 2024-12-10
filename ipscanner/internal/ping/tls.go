package ping

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/ipscanner/internal/statute"
)

type TlsPingResult struct {
	AddrPort   netip.AddrPort
	TLSVersion uint16
	RTT        time.Duration
	Err        error
}

func (t *TlsPingResult) Result() statute.IPInfo {
	return statute.IPInfo{AddrPort: t.AddrPort, RTT: t.RTT, CreatedAt: time.Now()}
}

func (t *TlsPingResult) Error() error {
	return t.Err
}

func (t *TlsPingResult) String() string {
	if t.Err != nil {
		return fmt.Sprintf("%s", t.Err)
	}

	return fmt.Sprintf("%s: protocol=%s, time=%d ms", t.AddrPort, statute.TlsVersionToString(t.TLSVersion), t.RTT)
}

type TlsPing struct {
	Host string
	Port uint16
	IP   netip.Addr

	opts *statute.ScannerOptions
}

func (t *TlsPing) Ping() statute.IPingResult {
	return t.PingContext(context.Background())
}

func (t *TlsPing) PingContext(ctx context.Context) statute.IPingResult {
	if !t.IP.IsValid() {
		return t.errorResult(errors.New("no IP specified"))
	}
	addr := netip.AddrPortFrom(t.IP, t.Port)
	t0 := time.Now()
	client, err := t.opts.TLSDialerFunc(ctx, "tcp", addr.String())
	if err != nil {
		return t.errorResult(err)
	}
	defer client.Close()
	return &TlsPingResult{AddrPort: addr, TLSVersion: t.opts.TlsVersion, RTT: time.Since(t0), Err: nil}
}

func NewTlsPing(ip netip.Addr, host string, port uint16, opts *statute.ScannerOptions) *TlsPing {
	return &TlsPing{
		IP:   ip,
		Host: host,
		Port: port,
		opts: opts,
	}
}

func (t *TlsPing) errorResult(err error) *TlsPingResult {
	r := &TlsPingResult{}
	r.Err = err
	return r
}

var (
	_ statute.IPing       = (*TlsPing)(nil)
	_ statute.IPingResult = (*TlsPingResult)(nil)
)
