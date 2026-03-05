package transport

import (
	"context"
	"net"
	"sync"
	"time"
)

type ConnectorCallbacks[T any] struct {
	IsClosed func(connection T) bool
	Close    func(connection T)
	Reset    func(connection T)
}

type Connector[T any] struct {
	dial      func(ctx context.Context) (T, error)
	callbacks ConnectorCallbacks[T]

	access           sync.Mutex
	connection       T
	hasConnection    bool
	connectionCancel context.CancelFunc
	connecting       chan struct{}

	closeCtx context.Context
	closed   bool
}

func NewConnector[T any](closeCtx context.Context, dial func(context.Context) (T, error), callbacks ConnectorCallbacks[T]) *Connector[T] {
	return &Connector[T]{
		dial:      dial,
		callbacks: callbacks,
		closeCtx:  closeCtx,
	}
}

func NewSingleflightConnector(closeCtx context.Context, dial func(context.Context) (*Connection, error)) *Connector[*Connection] {
	return NewConnector(closeCtx, dial, ConnectorCallbacks[*Connection]{
		IsClosed: func(connection *Connection) bool {
			return connection.IsClosed()
		},
		Close: func(connection *Connection) {
			connection.CloseWithError(ErrTransportClosed)
		},
		Reset: func(connection *Connection) {
			connection.CloseWithError(ErrConnectionReset)
		},
	})
}

type contextKeyConnecting struct{}

func (c *Connector[T]) Get(ctx context.Context) (T, error) {
	var zero T
	if ctx.Value(contextKeyConnecting{}) != nil {
		connection, _, err := c.dialWithCancellation(ctx)
		return connection, err
	}
	ctx = context.WithValue(ctx, contextKeyConnecting{}, true)
	for {
		c.access.Lock()

		if c.closed {
			c.access.Unlock()
			return zero, ErrTransportClosed
		}

		if c.hasConnection && !c.callbacks.IsClosed(c.connection) {
			connection := c.connection
			c.access.Unlock()
			return connection, nil
		}

		c.hasConnection = false
		if c.connectionCancel != nil {
			c.connectionCancel()
			c.connectionCancel = nil
		}

		if c.connecting != nil {
			connecting := c.connecting
			c.access.Unlock()

			select {
			case <-connecting:
				continue
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-c.closeCtx.Done():
				return zero, ErrTransportClosed
			}
		}

		c.connecting = make(chan struct{})
		c.access.Unlock()

		connection, cancel, err := c.dialWithCancellation(ctx)

		c.access.Lock()
		close(c.connecting)
		c.connecting = nil

		if err != nil {
			c.access.Unlock()
			return zero, err
		}

		if c.closed {
			cancel()
			c.callbacks.Close(connection)
			c.access.Unlock()
			return zero, ErrTransportClosed
		}

		c.connection = connection
		c.hasConnection = true
		c.connectionCancel = cancel
		result := c.connection
		c.access.Unlock()

		return result, nil
	}
}

func (c *Connector[T]) dialWithCancellation(ctx context.Context) (T, context.CancelFunc, error) {
	var zero T
	connCtx, cancel := context.WithCancel(c.closeCtx)

	dialDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-dialDone:
		}
	}()

	connection, err := c.dial(valueContext{connCtx, ctx})
	close(dialDone)
	if err != nil {
		cancel()
		return zero, nil, err
	}
	return connection, cancel, nil
}

type valueContext struct {
	context.Context
	parent context.Context
}

func (v valueContext) Value(key any) any {
	return v.parent.Value(key)
}

func (v valueContext) Deadline() (time.Time, bool) {
	return v.parent.Deadline()
}

func (c *Connector[T]) Close() error {
	c.access.Lock()
	defer c.access.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	if c.connectionCancel != nil {
		c.connectionCancel()
		c.connectionCancel = nil
	}
	if c.hasConnection {
		c.callbacks.Close(c.connection)
		c.hasConnection = false
	}

	return nil
}

func (c *Connector[T]) Reset() {
	c.access.Lock()
	defer c.access.Unlock()

	if c.connectionCancel != nil {
		c.connectionCancel()
		c.connectionCancel = nil
	}
	if c.hasConnection {
		c.callbacks.Reset(c.connection)
		c.hasConnection = false
	}
}

type Connection struct {
	net.Conn

	closeOnce  sync.Once
	done       chan struct{}
	closeError error
}

func WrapConnection(conn net.Conn) *Connection {
	return &Connection{
		Conn: conn,
		done: make(chan struct{}),
	}
}

func (c *Connection) Done() <-chan struct{} {
	return c.done
}

func (c *Connection) IsClosed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func (c *Connection) CloseError() error {
	select {
	case <-c.done:
		if c.closeError != nil {
			return c.closeError
		}
		return ErrTransportClosed
	default:
		return nil
	}
}

func (c *Connection) Close() error {
	return c.CloseWithError(ErrTransportClosed)
}

func (c *Connection) CloseWithError(err error) error {
	var returnError error
	c.closeOnce.Do(func() {
		c.closeError = err
		returnError = c.Conn.Close()
		close(c.done)
	})
	return returnError
}
