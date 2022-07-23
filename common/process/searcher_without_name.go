//go:build !cgo || !linux || android

package process

import (
	"context"
	"net/netip"
)

func FindProcessInfo(searcher Searcher, ctx context.Context, network string, srcIP netip.Addr, srcPort int) (*Info, error) {
	return searcher.FindProcessInfo(ctx, network, srcIP, srcPort)
}
