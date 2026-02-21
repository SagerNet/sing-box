package route

import (
	"context"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.ConnectionTracker = (*ActivityTracker)(nil)

type ActivityTracker struct {
	logger        logger.ContextLogger
	timeout       time.Duration
	checkInterval time.Duration // for testing
	lastActivity  atomic.Int64  // Unix nano timestamp
	done          chan struct{}
	exitFunc      func() // for testing
}

func NewActivityTracker(logger logger.ContextLogger, timeout time.Duration) *ActivityTracker {
	tracker := &ActivityTracker{
		logger:        logger,
		timeout:       timeout,
		checkInterval: 10 * time.Second,
		done:          make(chan struct{}),
		exitFunc: func() {
			os.Exit(0)
		},
	}
	tracker.lastActivity.Store(time.Now().UnixNano())
	return tracker
}

func (t *ActivityTracker) updateActivity(n int64) {
	if n > 0 {
		t.lastActivity.Store(time.Now().UnixNano())
	}
}

func (t *ActivityTracker) RoutedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) net.Conn {
	activityCounter := func(n int64) {
		t.updateActivity(n)
	}
	return bufio.NewCounterConn(conn,
		[]N.CountFunc{activityCounter},
		[]N.CountFunc{activityCounter})
}

func (t *ActivityTracker) RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) N.PacketConn {
	activityCounter := func(n int64) {
		t.updateActivity(n)
	}
	return bufio.NewCounterPacketConn(conn,
		[]N.CountFunc{activityCounter},
		[]N.CountFunc{activityCounter})
}

func (t *ActivityTracker) Start() error {
	go t.monitorActivity()
	return nil
}

func (t *ActivityTracker) Close() error {
	select {
	case <-t.done:
		return nil
	default:
		close(t.done)
	}
	return nil
}

func (t *ActivityTracker) monitorActivity() {
	ticker := time.NewTicker(t.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-t.done:
			return
		case <-ticker.C:
			lastActive := time.Unix(0, t.lastActivity.Load())
			idleDuration := time.Since(lastActive)

			if idleDuration >= t.timeout {
				t.logger.Info("idle timeout reached after ", idleDuration.String(), " of inactivity, exiting")
				t.exitFunc()
			}
		}
	}
}
