//go:build with_gvisor

package tailscale

import (
	"context"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/tailscale/ipn/ipnstate"
	"github.com/sagernet/tailscale/tailcfg"
)

func (t *Endpoint) StartTailscalePing(ctx context.Context, peerIP string, fn func(*adapter.TailscalePingResult)) error {
	ip, err := netip.ParseAddr(peerIP)
	if err != nil {
		return err
	}
	localClient, err := t.server.LocalClient()
	if err != nil {
		return err
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		result, pingErr := localClient.Ping(ctx, ip, tailcfg.PingDisco)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if pingErr != nil {
			fn(&adapter.TailscalePingResult{
				Error: pingErr.Error(),
			})
		} else {
			fn(convertPingResult(result))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func convertPingResult(result *ipnstate.PingResult) *adapter.TailscalePingResult {
	return &adapter.TailscalePingResult{
		LatencyMs:      result.LatencySeconds * 1000,
		IsDirect:       result.Endpoint != "",
		Endpoint:       result.Endpoint,
		DERPRegionID:   int32(result.DERPRegionID),
		DERPRegionCode: result.DERPRegionCode,
		Error:          result.Err,
	}
}
