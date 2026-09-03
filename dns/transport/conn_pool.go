package transport

import (
	"context"
	"net"
	"sync"

	"github.com/sagernet/sing/common/x/list"

	"golang.org/x/sync/semaphore"
)

type ConnPoolMode int

const (
	ConnPoolSingle ConnPoolMode = iota
	ConnPoolOrdered
)

type ConnPoolOptions[T comparable] struct {
	Mode ConnPoolMode
	// MaxInflight caps concurrent in-progress dials. Only honored in ConnPoolOrdered mode.
	MaxInflight int
	IsAlive     func(T) bool
	Close       func(T, error)
}

type ConnPool[T comparable] struct {
	options ConnPoolOptions[T]

	sem *semaphore.Weighted

	access sync.Mutex
	closed bool
	state  *connPoolState[T]
}

type connPoolState[T comparable] struct {
	ctx    context.Context
	cancel context.CancelCauseFunc

	all map[T]struct{}

	idle         list.List[T]
	idleElements map[T]*list.Element[T]

	shared        T
	hasShared     bool
	sharedClaimed bool
	sharedUsers   int
	sharedWaiters int
	sharedCtx     context.Context
	sharedCancel  context.CancelCauseFunc

	connecting *connPoolConnect[T]
}

type connPoolConnect[T comparable] struct {
	done chan struct{}
	err  error
}

func NewConnPool[T comparable](options ConnPoolOptions[T]) *ConnPool[T] {
	p := &ConnPool[T]{
		options: options,
	}
	if options.Mode == ConnPoolOrdered && options.MaxInflight > 0 {
		p.sem = semaphore.NewWeighted(int64(options.MaxInflight))
	}
	p.state = newConnPoolState[T](options.Mode)
	return p
}

func newConnPoolState[T comparable](mode ConnPoolMode) *connPoolState[T] {
	ctx, cancel := context.WithCancelCause(context.Background())
	state := &connPoolState[T]{
		ctx:    ctx,
		cancel: cancel,
		all:    make(map[T]struct{}),
	}
	if mode == ConnPoolOrdered {
		state.idleElements = make(map[T]*list.Element[T])
	}
	return state
}

func (p *ConnPool[T]) Acquire(ctx context.Context, dial func(context.Context) (T, error)) (T, bool, error) {
	switch p.options.Mode {
	case ConnPoolSingle:
		conn, _, created, err := p.acquireShared(ctx, dial)
		return conn, created, err
	case ConnPoolOrdered:
		return p.acquireOrdered(ctx, dial)
	default:
		var zero T
		return zero, false, net.ErrClosed
	}
}

func (p *ConnPool[T]) AcquireShared(ctx context.Context, dial func(context.Context) (T, error)) (T, context.Context, bool, error) {
	if p.options.Mode != ConnPoolSingle {
		var zero T
		return zero, nil, false, net.ErrClosed
	}
	return p.acquireShared(ctx, dial)
}

func (p *ConnPool[T]) Release(conn T, reuse bool) {
	p.access.Lock()
	if p.closed {
		p.access.Unlock()
		p.options.Close(conn, net.ErrClosed)
		return
	}
	state := p.state
	if _, tracked := state.all[conn]; !tracked {
		p.access.Unlock()
		p.options.Close(conn, net.ErrClosed)
		return
	}
	if !reuse || !p.options.IsAlive(conn) {
		p.removeConn(state, conn, net.ErrClosed)
		p.access.Unlock()
		p.options.Close(conn, net.ErrClosed)
		return
	}
	if p.options.Mode == ConnPoolSingle && state.hasShared && state.shared == conn && state.sharedUsers > 0 {
		state.sharedUsers--
	}
	if p.options.Mode == ConnPoolOrdered {
		if _, idle := state.idleElements[conn]; !idle {
			state.idleElements[conn] = state.idle.PushBack(conn)
		}
	}
	p.access.Unlock()
}

func (p *ConnPool[T]) Invalidate(conn T, cause error) {
	p.access.Lock()
	if p.closed {
		p.access.Unlock()
		p.options.Close(conn, cause)
		return
	}
	state := p.state
	if _, tracked := state.all[conn]; !tracked {
		p.access.Unlock()
		return
	}
	p.removeConn(state, conn, cause)
	p.access.Unlock()
	p.options.Close(conn, cause)
}

func (p *ConnPool[T]) acquireSlot(ctx context.Context, state *connPoolState[T]) error {
	if p.sem == nil {
		return nil
	}
	acquireCtx, cancel := context.WithCancel(ctx)
	stopStateCancel := context.AfterFunc(state.ctx, cancel)
	err := p.sem.Acquire(acquireCtx, 1)
	stopStateCancel()
	cancel()
	if err == nil {
		return nil
	}
	ctxErr := ctx.Err()
	if ctxErr != nil {
		return ctxErr
	}
	return context.Cause(state.ctx)
}

func (p *ConnPool[T]) releaseSlot() {
	if p.sem != nil {
		p.sem.Release(1)
	}
}

// removeConn must be called with p.access held.
func (p *ConnPool[T]) removeConn(state *connPoolState[T], conn T, cause error) {
	delete(state.all, conn)
	switch p.options.Mode {
	case ConnPoolSingle:
		if state.hasShared && state.shared == conn {
			var zero T
			state.shared = zero
			state.hasShared = false
			state.sharedClaimed = false
			state.sharedUsers = 0
			state.sharedCtx = nil
			if state.sharedCancel != nil {
				state.sharedCancel(cause)
				state.sharedCancel = nil
			}
		}
	case ConnPoolOrdered:
		if element, loaded := state.idleElements[conn]; loaded {
			state.idle.Remove(element)
			delete(state.idleElements, conn)
		}
	}
}

func (p *ConnPool[T]) CloseIdle() {
	p.access.Lock()
	if p.closed {
		p.access.Unlock()
		return
	}
	state := p.state
	if p.options.Mode != ConnPoolSingle || !state.hasShared || state.sharedUsers > 0 || state.sharedWaiters > 0 {
		p.access.Unlock()
		return
	}
	conn := state.shared
	p.removeConn(state, conn, net.ErrClosed)
	p.access.Unlock()
	p.options.Close(conn, net.ErrClosed)
}

func (p *ConnPool[T]) Reset() {
	p.access.Lock()
	if p.closed {
		p.access.Unlock()
		return
	}
	oldState := p.state
	p.state = newConnPoolState[T](p.options.Mode)
	p.access.Unlock()

	p.closeState(oldState, net.ErrClosed)
}

func (p *ConnPool[T]) Close() error {
	p.access.Lock()
	if p.closed {
		p.access.Unlock()
		return nil
	}
	p.closed = true
	oldState := p.state
	p.state = nil
	p.access.Unlock()

	p.closeState(oldState, net.ErrClosed)
	return nil
}

func (p *ConnPool[T]) acquireOrdered(ctx context.Context, dial func(context.Context) (T, error)) (T, bool, error) {
	var zero T
	for {
		p.access.Lock()
		if p.closed {
			p.access.Unlock()
			return zero, false, net.ErrClosed
		}
		current := p.state
		if element := current.idle.Front(); element != nil {
			idleConn := current.idle.Remove(element)
			delete(current.idleElements, idleConn)
			if p.options.IsAlive(idleConn) {
				p.access.Unlock()
				return idleConn, false, nil
			}
			delete(current.all, idleConn)
			p.access.Unlock()
			p.options.Close(idleConn, net.ErrClosed)
			continue
		}
		p.access.Unlock()
		return p.dialAndInstall(ctx, current, dial)
	}
}

func (p *ConnPool[T]) dialAndInstall(ctx context.Context, current *connPoolState[T], dial func(context.Context) (T, error)) (T, bool, error) {
	var zero T
	err := p.acquireSlot(ctx, current)
	if err != nil {
		return zero, false, err
	}
	defer p.releaseSlot()
	dialCtx, dialCancel := context.WithCancelCause(ctx)
	stopStateCancel := context.AfterFunc(current.ctx, func() {
		dialCancel(context.Cause(current.ctx))
	})
	conn, err := dial(dialCtx)
	stateCancelStopped := stopStateCancel()
	dialErr := context.Cause(dialCtx)
	if dialErr == nil && !stateCancelStopped {
		dialErr = context.Cause(current.ctx)
	}
	dialCancel(nil)
	if err != nil {
		if dialErr != nil {
			return zero, false, dialErr
		}
		return zero, false, err
	}
	if dialErr != nil {
		p.options.Close(conn, dialErr)
		return zero, false, dialErr
	}

	p.access.Lock()
	if p.closed {
		p.access.Unlock()
		p.options.Close(conn, net.ErrClosed)
		return zero, false, net.ErrClosed
	}
	if p.state != current {
		p.access.Unlock()
		p.options.Close(conn, net.ErrClosed)
		return zero, false, net.ErrClosed
	}
	current.all[conn] = struct{}{}
	p.access.Unlock()
	return conn, true, nil
}

func (p *ConnPool[T]) acquireShared(ctx context.Context, dial func(context.Context) (T, error)) (T, context.Context, bool, error) {
	var zero T
	for {
		p.access.Lock()
		if p.closed {
			p.access.Unlock()
			return zero, nil, false, net.ErrClosed
		}
		current := p.state
		if current.hasShared {
			conn := current.shared
			if p.options.IsAlive(conn) {
				created := !current.sharedClaimed
				current.sharedClaimed = true
				current.sharedUsers++
				connCtx := current.sharedCtx
				p.access.Unlock()
				return conn, connCtx, created, nil
			}
			p.removeConn(current, conn, net.ErrClosed)
			p.access.Unlock()
			p.options.Close(conn, net.ErrClosed)
			continue
		}

		startDial := current.connecting == nil
		if startDial {
			current.connecting = &connPoolConnect[T]{done: make(chan struct{})}
		}
		state := current.connecting
		current.sharedWaiters++
		p.access.Unlock()

		if startDial {
			go p.connectSingle(current, state, ctx, dial)
		}

		select {
		case <-state.done:
			conn, connCtx, created, retry, err := p.collectShared(current, state, startDial)
			if retry {
				continue
			}
			return conn, connCtx, created, err
		case <-ctx.Done():
			p.access.Lock()
			current.sharedWaiters--
			p.access.Unlock()
			return zero, nil, false, ctx.Err()
		case <-current.ctx.Done():
			p.access.Lock()
			current.sharedWaiters--
			closed := p.closed
			p.access.Unlock()
			if closed {
				return zero, nil, false, net.ErrClosed
			}
		}
	}
}

func (p *ConnPool[T]) connectSingle(current *connPoolState[T], state *connPoolConnect[T], ctx context.Context, dial func(context.Context) (T, error)) {
	dialCtx, dialCancel := context.WithCancelCause(ctx)
	stopStateCancel := context.AfterFunc(current.ctx, func() {
		dialCancel(context.Cause(current.ctx))
	})
	conn, err := dial(dialCtx)
	stateCancelStopped := stopStateCancel()
	dialErr := context.Cause(dialCtx)
	if dialErr == nil && !stateCancelStopped {
		dialErr = context.Cause(current.ctx)
	}
	dialCancel(nil)
	if dialErr != nil {
		if err == nil {
			p.options.Close(conn, dialErr)
		}
		err = dialErr
	}

	var closeErr error
	p.access.Lock()
	current.connecting = nil
	if err != nil {
		state.err = err
	} else if p.closed {
		closeErr = net.ErrClosed
		state.err = closeErr
	} else if p.state != current {
		closeErr = net.ErrClosed
		state.err = closeErr
	} else {
		sharedCtx, sharedCancel := context.WithCancelCause(current.ctx)
		current.shared = conn
		current.hasShared = true
		current.sharedCtx = sharedCtx
		current.sharedCancel = sharedCancel
		current.all[conn] = struct{}{}
	}
	p.access.Unlock()

	if closeErr != nil {
		p.options.Close(conn, closeErr)
	}
	close(state.done)
}

func (p *ConnPool[T]) collectShared(current *connPoolState[T], state *connPoolConnect[T], startDial bool) (T, context.Context, bool, bool, error) {
	var zero T

	p.access.Lock()
	current.sharedWaiters--
	if state.err != nil {
		err := state.err
		p.access.Unlock()
		if startDial {
			return zero, nil, false, false, err
		}
		return zero, nil, false, true, nil
	}
	if p.closed {
		p.access.Unlock()
		return zero, nil, false, false, net.ErrClosed
	}
	if p.state != current {
		p.access.Unlock()
		return zero, nil, false, false, net.ErrClosed
	}
	if !current.hasShared {
		p.access.Unlock()
		return zero, nil, false, true, nil
	}

	conn := current.shared
	if !p.options.IsAlive(conn) {
		p.removeConn(current, conn, net.ErrClosed)
		p.access.Unlock()
		p.options.Close(conn, net.ErrClosed)
		return zero, nil, false, true, nil
	}

	created := !current.sharedClaimed
	current.sharedClaimed = true
	current.sharedUsers++
	connCtx := current.sharedCtx
	p.access.Unlock()
	return conn, connCtx, created, false, nil
}

func (p *ConnPool[T]) closeState(state *connPoolState[T], cause error) {
	state.cancel(cause)
	for conn := range state.all {
		p.options.Close(conn, cause)
	}
}
