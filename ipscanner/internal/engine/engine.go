package engine

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"

	"github.com/sagernet/sing-box/ipscanner/internal/iterator"
	"github.com/sagernet/sing-box/ipscanner/internal/ping"
	"github.com/sagernet/sing-box/ipscanner/internal/statute"
)

type Engine struct {
	generator *iterator.IpGenerator
	ipQueue   *IPQueue
	ping      func(context.Context, netip.Addr) (statute.IPInfo, error)
	log       *slog.Logger
}

func NewScannerEngine(opts *statute.ScannerOptions) *Engine {
	queue := NewIPQueue(opts)

	p := ping.Ping{
		Options: opts,
	}
	return &Engine{
		ipQueue:   queue,
		ping:      p.DoPing,
		generator: iterator.NewIterator(opts),
		log:       opts.Logger,
	}
}

func (e *Engine) GetAvailableIPs(desc bool) []statute.IPInfo {
	if e.ipQueue != nil {
		return e.ipQueue.AvailableIPs(desc)
	}
	return nil
}

func (e *Engine) Run(ctx context.Context) {
	e.ipQueue.Init()

	select {
	case <-ctx.Done():
		return
	case <-e.ipQueue.available:
		e.log.Debug("Started new scanning round")
		batch, err := e.generator.NextBatch()
		if err != nil {
			e.log.Error("Error while generating IP: %v", err)
			return
		}
		for _, ip := range batch {
			select {
			case <-ctx.Done():
				return
			default:
				ipInfo, err := e.ping(ctx, ip)
				if err != nil {
					if !errors.Is(err, context.Canceled) {
						e.log.Error("ping error", "addr", ip, "error", err)
					}
					continue
				}
				e.log.Debug("ping success", "addr", ipInfo.AddrPort, "rtt", ipInfo.RTT)
				e.ipQueue.Enqueue(ipInfo)
			}
		}
	}
}
