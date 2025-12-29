package transport

import (
	"context"
	"os"
	"sync"

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

	mutex           sync.Mutex
	state           TransportState
	inFlight        int32
	queriesComplete chan struct{}
	closeCtx        context.Context
	closeCancel     context.CancelFunc
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
	if t.inFlight > 0 {
		t.inFlight--
	}
	if t.inFlight == 0 && t.queriesComplete != nil {
		close(t.queriesComplete)
		t.queriesComplete = nil
	}
	t.mutex.Unlock()
}

func (t *BaseTransport) CloseContext() context.Context {
	return t.closeCtx
}

func (t *BaseTransport) Shutdown(ctx context.Context) error {
	t.mutex.Lock()

	if t.state >= StateClosing {
		t.mutex.Unlock()
		return nil
	}

	if t.state == StateNew {
		t.state = StateClosed
		t.mutex.Unlock()
		t.closeCancel()
		return nil
	}

	t.state = StateClosing

	if t.inFlight == 0 {
		t.state = StateClosed
		t.mutex.Unlock()
		t.closeCancel()
		return nil
	}

	t.queriesComplete = make(chan struct{})
	queriesComplete := t.queriesComplete
	t.mutex.Unlock()

	t.closeCancel()

	select {
	case <-queriesComplete:
		t.mutex.Lock()
		t.state = StateClosed
		t.mutex.Unlock()
		return nil
	case <-ctx.Done():
		t.mutex.Lock()
		t.state = StateClosed
		t.mutex.Unlock()
		return ctx.Err()
	}
}

func (t *BaseTransport) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), C.TCPTimeout)
	defer cancel()
	return t.Shutdown(ctx)
}
