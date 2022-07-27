package pipelistener

import (
	"io"
	"net"
)

var _ net.Listener = (*Listener)(nil)

type Listener struct {
	pipe chan net.Conn
	done chan struct{}
}

func New(channelSize int) *Listener {
	return &Listener{
		pipe: make(chan net.Conn, channelSize),
		done: make(chan struct{}),
	}
}

func (l *Listener) Serve(conn net.Conn) {
	l.pipe <- conn
}

func (l *Listener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.pipe:
		return conn, nil
	case <-l.done:
		return nil, io.ErrClosedPipe
	}
}

func (l *Listener) Close() error {
	select {
	case <-l.done:
		return io.ErrClosedPipe
	default:
	}
	close(l.done)
	return nil
}

func (l *Listener) Addr() net.Addr {
	return addr{}
}

type addr struct{}

func (a addr) Network() string {
	return "pipe"
}

func (a addr) String() string {
	return "pipe"
}
