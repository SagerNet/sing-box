package route

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/byteformats"
	N "github.com/sagernet/sing/common/network"
)

var (
	_ tun.FlowTracker = (*flowLogger)(nil)
	_ tun.FlowTracker = (*flowInterrupter)(nil)
	_ tun.FlowTracker = multiFlowTracker(nil)
)

type flowLogger struct {
	ctx         context.Context
	logger      log.ContextLogger
	network     string
	source      string
	destination string
	outbound    adapter.Outbound
	createdAt   time.Time
	upload      atomic.Int64
	download    atomic.Int64
}

func newFlowLogger(ctx context.Context, logger log.ContextLogger, metadata adapter.InboundContext, outbound adapter.Outbound) *flowLogger {
	var source, destination string
	if metadata.Network == N.NetworkICMP {
		source = metadata.Source.AddrString()
		destination = metadata.Destination.AddrString()
	} else {
		source = metadata.Source.String()
		destination = metadata.Destination.String()
	}
	return &flowLogger{
		ctx:         ctx,
		logger:      logger,
		network:     metadata.Network,
		source:      source,
		destination: destination,
		outbound:    outbound,
	}
}

func (l *flowLogger) AttachFlow(handle tun.FlowHandle) {
	l.createdAt = time.Now()
}

func (l *flowLogger) CountForward(n int) {
	l.upload.Add(int64(n))
}

func (l *flowLogger) CountReverse(n int) {
	l.download.Add(int64(n))
}

func (l *flowLogger) FlowEstablished() {
}

func (l *flowLogger) CloseFlow(reason tun.FlowCloseReason) {
	l.logger.DebugContext(l.ctx, "flow closed: ", reason,
		", upload ", byteformats.FormatBytes(uint64(l.upload.Load())), ", download ", byteformats.FormatBytes(uint64(l.download.Load())))
}

type multiFlowTracker []tun.FlowTracker

func (t multiFlowTracker) AttachFlow(handle tun.FlowHandle) {
	for _, tracker := range t {
		tracker.AttachFlow(handle)
	}
}

func (t multiFlowTracker) CountForward(n int) {
	for _, tracker := range t {
		tracker.CountForward(n)
	}
}

func (t multiFlowTracker) CountReverse(n int) {
	for _, tracker := range t {
		tracker.CountReverse(n)
	}
}

func (t multiFlowTracker) FlowEstablished() {
	for _, tracker := range t {
		tracker.FlowEstablished()
	}
}

func (t multiFlowTracker) CloseFlow(reason tun.FlowCloseReason) {
	for _, tracker := range t {
		tracker.CloseFlow(reason)
	}
}

type flowInterrupter struct {
	groups   []adapter.OutboundGroup
	removers []func()
}

func newFlowInterrupter(chain []adapter.Outbound) *flowInterrupter {
	groups := common.FilterIsInstance(chain, func(it adapter.Outbound) (adapter.OutboundGroup, bool) {
		group, isGroup := it.(adapter.OutboundGroup)
		return group, isGroup
	})
	if len(groups) == 0 {
		return nil
	}
	return &flowInterrupter{groups: groups}
}

type flowCloser struct {
	tun.FlowHandle
}

func (c flowCloser) Close() error {
	c.CloseFlow()
	return nil
}

func (t *flowInterrupter) AttachFlow(handle tun.FlowHandle) {
	closer := flowCloser{handle}
	for _, group := range t.groups {
		t.removers = append(t.removers, group.AttachConnection(closer))
	}
}

func (t *flowInterrupter) CountForward(n int) {
}

func (t *flowInterrupter) CountReverse(n int) {
}

func (t *flowInterrupter) FlowEstablished() {
}

func (t *flowInterrupter) CloseFlow(reason tun.FlowCloseReason) {
	for _, remove := range t.removers {
		remove()
	}
}
