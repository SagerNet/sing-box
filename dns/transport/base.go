package transport

import (
	"context"
	"os"
	"sync"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
)

type TransportState int

const (
	StateNew TransportState = iota
	StateStarted
	StateClosing
	StateClosed
)

var (
	ErrTransportClosed = os.ErrClosed
	ErrConnectionReset = E.New("connection reset")
)

type BaseTransport struct {
	dns.TransportAdapter
	Logger logger.ContextLogger

	mutex       sync.Mutex
	state       TransportState
	inFlight    int
	closeCtx    context.Context
	closeCancel context.CancelFunc
	drainSignal chan struct{}
}

func NewBaseTransport(adapter dns.TransportAdapter, logger logger.ContextLogger) *BaseTransport {
	ctx, cancel := context.WithCancel(context.Background())
	return &BaseTransport{
		TransportAdapter: adapter,
		Logger:           logger,
		state:            StateNew,
		closeCtx:         ctx,
		closeCancel:      cancel,
	}
}

func (t *BaseTransport) State() TransportState {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.state
}

func (t *BaseTransport) SetStarted() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	switch t.state {
	case StateNew:
		t.state = StateStarted
		return nil
	case StateStarted:
		return nil
	default:
		return ErrTransportClosed
	}
}

func (t *BaseTransport) BeginQuery() bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.state != StateStarted {
		return false
	}
	t.inFlight++
	return true
}

func (t *BaseTransport) EndQuery() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.inFlight > 0 {
		t.inFlight--
	}
	if t.inFlight == 0 && t.drainSignal != nil {
		close(t.drainSignal)
		t.drainSignal = nil
	}
}

func (t *BaseTransport) CloseContext() context.Context {
	return t.closeCtx
}

func (t *BaseTransport) Shutdown(ctx context.Context) error {
	t.mutex.Lock()

	if t.state == StateClosed {
		t.mutex.Unlock()
		return nil
	}

	if t.state == StateClosing {
		sig := t.drainSignal
		t.mutex.Unlock()
		if sig == nil {
			return nil
		}
		select {
		case <-sig:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if t.state == StateNew {
		t.state = StateClosed
		t.mutex.Unlock()
		t.closeCancel()
		return nil
	}

	t.state = StateClosing
	t.closeCancel()

	if t.inFlight == 0 {
		t.state = StateClosed
		t.mutex.Unlock()
		return nil
	}

	if t.drainSignal == nil {
		t.drainSignal = make(chan struct{})
	}
	sig := t.drainSignal
	t.mutex.Unlock()

	select {
	case <-sig:
		t.mutex.Lock()
		t.state = StateClosed
		t.mutex.Unlock()
		return nil
	case <-ctx.Done():
		t.mutex.Lock()
		t.state = StateClosed
		inFlight := t.inFlight
		t.mutex.Unlock()
		if inFlight > 0 {
			t.Logger.WarnContext(ctx, "shutdown timed out while waiting for ", inFlight, " in-flight queries to drain")
		}
		return nil
	}
}

func (t *BaseTransport) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), C.StopTimeout)
	defer cancel()
	return t.Shutdown(ctx)
}

func (t *BaseTransport) ContextWithCancel(ctx context.Context) (context.Context, context.CancelFunc) {
	connCtx, cancel := context.WithCancel(t.closeCtx)
	stop := context.AfterFunc(ctx, func() {
		cancel()
	})
	return joinedContext{connCtx, ctx}, func() {
		stop()
		cancel()
	}
}

// joinedContext wraps a background context (providing cancellation)
// with a parent context (providing values and deadlines).
// This decouples the IO timeout/cancellation source from the metadata.
type joinedContext struct {
	context.Context
	parent context.Context
}

func (v joinedContext) Value(key any) any {
	return v.parent.Value(key)
}

func (v joinedContext) Deadline() (time.Time, bool) {
	return v.parent.Deadline()
}
