package ping

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/ipscanner/internal/statute"
)

type TcpPingResult struct {
	AddrPort netip.AddrPort
	RTT      time.Duration
	Err      error
}

func (tp *TcpPingResult) Result() statute.IPInfo {
	return statute.IPInfo{AddrPort: tp.AddrPort, RTT: tp.RTT, CreatedAt: time.Now()}
}

func (tp *TcpPingResult) Error() error {
	return tp.Err
}

func (tp *TcpPingResult) String() string {
	if tp.Err != nil {
		return fmt.Sprintf("%s", tp.Err)
	} else {
		return fmt.Sprintf("%s: time=%d ms", tp.AddrPort, tp.RTT)
	}
}

type TcpPing struct {
	host string
	port uint16
	ip   netip.Addr

	opts statute.ScannerOptions
}

func (tp *TcpPing) SetHost(host string) {
	tp.host = host
	tp.ip, _ = netip.ParseAddr(host)
}

func (tp *TcpPing) Host() string {
	return tp.host
}

func (tp *TcpPing) Ping() statute.IPingResult {
	return tp.PingContext(context.Background())
}

func (tp *TcpPing) PingContext(ctx context.Context) statute.IPingResult {
	if !tp.ip.IsValid() {
		return &TcpPingResult{AddrPort: netip.AddrPort{}, RTT: 0, Err: errors.New("no IP specified")}
	}

	addr := netip.AddrPortFrom(tp.ip, tp.port)
	t0 := time.Now()
	conn, err := tp.opts.RawDialerFunc(ctx, "tcp", addr.String())
	if err != nil {
		return &TcpPingResult{AddrPort: addr, RTT: 0, Err: err}
	}
	defer conn.Close()

	return &TcpPingResult{AddrPort: addr, RTT: time.Since(t0), Err: nil}
}

func NewTcpPing(ip netip.Addr, host string, port uint16, opts *statute.ScannerOptions) *TcpPing {
	return &TcpPing{
		host: host,
		port: port,
		ip:   ip,
		opts: *opts,
	}
}

var (
	_ statute.IPing       = (*TcpPing)(nil)
	_ statute.IPingResult = (*TcpPingResult)(nil)
)
