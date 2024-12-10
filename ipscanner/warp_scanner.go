package ipscanner

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"os"
	"time"

	"github.com/sagernet/sing-box/warp"
)

var googlev6DNSAddr80 = netip.MustParseAddrPort("[2001:4860:4860::8888]:80")

type WarpScanOptions struct {
	PrivateKey string
	PublicKey  string
	MaxRTT     time.Duration
	V4         bool
	V6         bool
	Port       uint16
}

func findMinRTT(ipInfos []IPInfo) (IPInfo, error) {
	if len(ipInfos) == 0 {
		return IPInfo{}, errors.New("list is empty")
	}

	minRTTInfo := ipInfos[0]
	for _, ipInfo := range ipInfos[1:] {
		if ipInfo.RTT < minRTTInfo.RTT {
			minRTTInfo = ipInfo
		}
	}

	return minRTTInfo, nil
}

func RunWarpScan(ctx context.Context, opts WarpScanOptions) (result IPInfo, err error) {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	scanner := NewScanner(
		WithLogger(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))),
		WithWarpPing(),
		WithWarpPrivateKey(opts.PrivateKey),
		WithWarpPeerPublicKey(opts.PublicKey),
		WithUseIPv4(opts.V4),
		WithUseIPv6(CanConnectIPv6(googlev6DNSAddr80)),
		WithMaxDesirableRTT(opts.MaxRTT),
		WithCidrList(warp.WarpPrefixes()),
		WithPort(opts.Port),
	)

	scanner.Run(ctx)

	t := time.NewTicker(1 * time.Second)
	defer t.Stop()

	for {
		ipList := scanner.GetAvailableIPs()
		if len(ipList) > 1 {
			bestIp, err := findMinRTT(ipList)
			if err != nil {
				return IPInfo{}, err
			}
			return bestIp, nil
		}

		select {
		case <-ctx.Done():
			// Context is done - canceled externally
			return IPInfo{}, errors.New("user canceled the operation")
		case <-t.C:
			// Prevent the loop from spinning too fast
			continue
		}
	}
}
