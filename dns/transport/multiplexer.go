package transport

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"

	E "github.com/sagernet/sing/common/exceptions"

	mDNS "github.com/miekg/dns"
)

type queryMultiplexerOptions struct {
	dial           func(ctx context.Context) (net.Conn, error)
	write          func(conn net.Conn, message *mDNS.Msg, queryId uint16) error
	readNext       func(conn net.Conn) (*mDNS.Msg, error)
	retryReadError bool
}

type queryMultiplexer struct {
	options    queryMultiplexerOptions
	connection *ConnPool[*multiplexConn]

	queryAccess sync.Mutex
	queryId     uint16
	queries     map[uint16]*pendingQuery
}

type multiplexConn struct {
	net.Conn
	readEpoch atomic.Uint64
}

type queryMultiplexerReadError struct {
	cause error
}

func (e *queryMultiplexerReadError) Error() string {
	return e.cause.Error()
}

func (e *queryMultiplexerReadError) Unwrap() error {
	return e.cause
}

type pendingQuery struct {
	conn        *multiplexConn
	message     *mDNS.Msg
	readEpoch   uint64
	callback    func(response *mDNS.Msg, err error)
	stopContext func() bool
	stopConn    func() bool
	retryCtx    context.Context
}

func newQueryMultiplexer(options queryMultiplexerOptions) *queryMultiplexer {
	return &queryMultiplexer{
		options: options,
		queries: make(map[uint16]*pendingQuery),
		connection: NewConnPool(ConnPoolOptions[*multiplexConn]{
			Mode: ConnPoolSingle,
			IsAlive: func(conn *multiplexConn) bool {
				return conn != nil
			},
			Close: func(conn *multiplexConn, cause error) {
				conn.Close()
			},
		}),
	}
}

func (m *queryMultiplexer) Close() error {
	return m.connection.Close()
}

func (m *queryMultiplexer) Reset() {
	m.connection.Reset()
}

func (m *queryMultiplexer) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	done := make(chan struct{})
	var (
		response *mDNS.Msg
		err      error
	)
	m.ExchangeAsync(ctx, message, func(callbackResponse *mDNS.Msg, callbackErr error) {
		response = callbackResponse
		err = callbackErr
		close(done)
	})
	<-done
	return response, err
}

func (m *queryMultiplexer) ExchangeAsync(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error)) {
	m.exchangeAsync(ctx, message, callback, true)
}

func (m *queryMultiplexer) exchangeAsync(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error), retryReadError bool) {
	for firstAttempt := true; ; firstAttempt = false {
		conn, connCtx, created, err := m.connection.AcquireShared(ctx, m.dialConn)
		if err != nil {
			callback(nil, err)
			return
		}
		if created {
			go m.recvLoop(conn)
		}
		queryId, err := m.register(ctx, connCtx, conn, message, callback, retryReadError && m.options.retryReadError && !created)
		if err != nil {
			m.connection.Release(conn, true)
			callback(nil, err)
			return
		}
		writeErr := m.options.write(conn, message, queryId)
		if writeErr == nil {
			return
		}
		pending := m.take(queryId)
		m.connection.Invalidate(conn, writeErr)
		if pending == nil {
			return
		}
		if !created && firstAttempt {
			continue
		}
		callback(nil, E.Cause(writeErr, "write request"))
		return
	}
}

func (m *queryMultiplexer) dialConn(ctx context.Context) (*multiplexConn, error) {
	conn, err := m.options.dial(ctx)
	if err != nil {
		return nil, err
	}
	return &multiplexConn{Conn: conn}, nil
}

func (m *queryMultiplexer) register(ctx context.Context, connCtx context.Context, conn *multiplexConn, message *mDNS.Msg, callback func(response *mDNS.Msg, err error), retryReadError bool) (uint16, error) {
	m.queryAccess.Lock()
	defer m.queryAccess.Unlock()
	start := m.queryId
	for {
		m.queryId++
		if _, exists := m.queries[m.queryId]; !exists {
			break
		}
		if m.queryId == start {
			return 0, E.New("no available query ID")
		}
	}
	queryId := m.queryId
	pending := &pendingQuery{
		conn:      conn,
		message:   message,
		readEpoch: conn.readEpoch.Load(),
		callback:  callback,
	}
	if retryReadError {
		pending.retryCtx = ctx
	}
	m.queries[queryId] = pending
	pending.stopContext = context.AfterFunc(ctx, func() {
		m.completeContextDone(queryId, ctx)
	})
	pending.stopConn = context.AfterFunc(connCtx, func() {
		m.completeConnDone(queryId, connCtx)
	})
	return queryId, nil
}

func (m *queryMultiplexer) completeConnDone(queryId uint16, connCtx context.Context) {
	pending := m.take(queryId)
	if pending == nil {
		return
	}
	connErr := context.Cause(connCtx)
	_, readFailed := connErr.(*queryMultiplexerReadError)
	if pending.retryCtx != nil && readFailed {
		m.exchangeAsync(pending.retryCtx, pending.message, pending.callback, false)
		return
	}
	pending.callback(nil, connErr)
}

func (m *queryMultiplexer) take(queryId uint16) *pendingQuery {
	m.queryAccess.Lock()
	pending, loaded := m.queries[queryId]
	if !loaded {
		m.queryAccess.Unlock()
		return nil
	}
	delete(m.queries, queryId)
	m.queryAccess.Unlock()
	pending.stopContext()
	pending.stopConn()
	return pending
}

func (m *queryMultiplexer) complete(queryId uint16, response *mDNS.Msg, err error, releaseConn bool) {
	pending := m.take(queryId)
	if pending == nil {
		return
	}
	if releaseConn {
		m.connection.Release(pending.conn, true)
	}
	if response != nil {
		response.Id = pending.message.Id
	}
	pending.callback(response, err)
}

func (m *queryMultiplexer) completeContextDone(queryId uint16, ctx context.Context) {
	pending := m.take(queryId)
	if pending == nil {
		return
	}
	err := ctx.Err()
	if errors.Is(err, context.DeadlineExceeded) && pending.conn.readEpoch.Load() == pending.readEpoch {
		m.connection.Invalidate(pending.conn, err)
	} else {
		m.connection.Release(pending.conn, true)
	}
	pending.callback(nil, err)
}

func (m *queryMultiplexer) recvLoop(conn *multiplexConn) {
	for {
		message, err := m.options.readNext(conn)
		if err != nil {
			m.connection.Invalidate(conn, &queryMultiplexerReadError{cause: err})
			return
		}
		conn.readEpoch.Add(1)
		if message == nil {
			continue
		}
		m.complete(message.Id, message, nil, true)
	}
}
