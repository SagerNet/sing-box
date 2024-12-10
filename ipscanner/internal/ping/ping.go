package ping

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/sagernet/sing-box/ipscanner/internal/statute"
)

type Ping struct {
	Options *statute.ScannerOptions
}

// DoPing performs a ping on the given IP address.
func (p *Ping) DoPing(ctx context.Context, ip netip.Addr) (statute.IPInfo, error) {
	if p.Options.SelectedOps&statute.HTTPPing > 0 {
		res, err := p.httpPing(ctx, ip)
		if err != nil {
			return statute.IPInfo{}, err
		}

		return res, nil
	}
	if p.Options.SelectedOps&statute.TLSPing > 0 {
		res, err := p.tlsPing(ctx, ip)
		if err != nil {
			return statute.IPInfo{}, err
		}

		return res, nil
	}
	if p.Options.SelectedOps&statute.TCPPing > 0 {
		res, err := p.tcpPing(ctx, ip)
		if err != nil {
			return statute.IPInfo{}, err
		}

		return res, nil
	}
	if p.Options.SelectedOps&statute.WARPPing > 0 {
		res, err := p.warpPing(ctx, ip)
		if err != nil {
			return statute.IPInfo{}, err
		}

		return res, nil
	}

	return statute.IPInfo{}, errors.New("no ping operation selected")
}

func (p *Ping) httpPing(ctx context.Context, ip netip.Addr) (statute.IPInfo, error) {
	return p.calc(
		ctx,
		NewHttpPing(
			ip,
			"GET",
			fmt.Sprintf(
				"https://%s:%d%s",
				p.Options.Hostname,
				p.Options.Port,
				p.Options.HTTPPath,
			),
			p.Options,
		),
	)
}

func (p *Ping) warpPing(ctx context.Context, ip netip.Addr) (statute.IPInfo, error) {
	return p.calc(ctx, NewWarpPing(ip, p.Options))
}

func (p *Ping) tlsPing(ctx context.Context, ip netip.Addr) (statute.IPInfo, error) {
	return p.calc(ctx,
		NewTlsPing(ip, p.Options.Hostname, p.Options.Port, p.Options),
	)
}

func (p *Ping) tcpPing(ctx context.Context, ip netip.Addr) (statute.IPInfo, error) {
	return p.calc(ctx,
		NewTcpPing(ip, p.Options.Hostname, p.Options.Port, p.Options),
	)
}

func (p *Ping) calc(ctx context.Context, tp statute.IPing) (statute.IPInfo, error) {
	pr := tp.PingContext(ctx)
	err := pr.Error()
	if err != nil {
		return statute.IPInfo{}, err
	}
	return pr.Result(), nil
}
