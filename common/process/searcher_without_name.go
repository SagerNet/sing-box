//go:build !linux || android

package process

import (
	"context"
	"net/netip"
)

func findProcessInfo(searcher Searcher, ctx context.Context, network string, source netip.AddrPort, destination netip.AddrPort) (*Info, error) {
	return searcher.FindProcessInfo(ctx, network, source, destination)
}
