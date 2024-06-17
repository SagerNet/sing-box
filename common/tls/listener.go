package tls

import (
	"context"
	"net"

	"github.com/sagernet/sing/common/tls"
)

var _ net.Listener = (*Listener)(nil)

type Listener struct {
	ctx context.Context
	l   net.Listener
	tls tls.ServerConfig
}

func (t *Listener) Addr() net.Addr {
	return t.l.Addr()
}

func (t *Listener) Accept() (net.Conn, error) {
	if t.tls == nil {
		return t.l.Accept()
	}
	for {
		conn, err := t.l.Accept()
		if err != nil {
			return nil, err
		}
		conn, err = tls.ServerHandshake(t.ctx, conn, t.tls)
		if err == nil {
			return conn, nil
		}
	}
}

func (t *Listener) Close() error {
	if err := t.l.Close(); err != nil {
		return err
	}
	if t.tls != nil {
		return t.tls.Close()
	}
	return nil
}

func NewListener(ctx context.Context, address string, config ServerConfig) (net.Listener, error) {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	return &Listener{
		ctx: ctx,
		l:   l,
		tls: config,
	}, nil
}
