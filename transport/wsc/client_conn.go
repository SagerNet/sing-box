package wsc

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/network"
	"github.com/sagernet/ws"
	"github.com/sagernet/ws/wsutil"
)

var _ network.ExtendedConn = &clientConn{}

type clientConn struct {
	net.Conn
	reader *wsutil.Reader
	buf    [2048]byte
	mu     sync.Mutex
}

func (cli *Client) newConn(ctx context.Context, network string, endpoint string) (*clientConn, error) {
	conn, err := cli.newWSConn(ctx, network, endpoint)
	if err != nil {
		return nil, err
	}
	reader := wsutil.NewReader(conn, ws.StateClientSide)
	return &clientConn{
		Conn:   conn,
		reader: reader,
	}, nil
}

func (conn *clientConn) Close() error {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	_ = wsutil.WriteClientMessage(conn.Conn, ws.OpClose, nil)
	return conn.Conn.Close()
}

func (conn *clientConn) ReadBuffer(buffer *buf.Buffer) error {
	if buffer == nil {
		return errors.New("buffer is nil")
	}
	n, err := conn.Read(conn.buf[:])
	if _, wErr := buffer.Write(conn.buf[:n]); wErr != nil {
		return wErr
	}
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func (conn *clientConn) WriteBuffer(buffer *buf.Buffer) error {
	if buffer == nil {
		return errors.New("buffer is nil")
	}
	conn.mu.Lock()
	defer conn.mu.Unlock()
	return wsutil.WriteClientBinary(conn.Conn, buffer.Bytes())
}

func (conn *clientConn) Read(b []byte) (n int, err error) {
	err = nil
	var header ws.Header
	for {
		n, err = conn.reader.Read(b)
		if n > 0 {
			return
		}

		if !exceptions.IsMulti(err, io.EOF, wsutil.ErrNoFrameAdvance) {
			return
		}

		header, err = conn.reader.NextFrame()
		if err != nil {
			return
		}

		switch header.OpCode {
		case ws.OpBinary, ws.OpText, ws.OpContinuation:
			continue
		case ws.OpPing:
			wsutil.WriteClientMessage(conn.Conn, ws.OpPong, nil)
		case ws.OpPong:
			continue
		case ws.OpClose:
			err = io.EOF
			return
		default:
			continue
		}
	}
}

func (conn *clientConn) Write(b []byte) (n int, err error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if err := wsutil.WriteClientBinary(conn.Conn, b); err != nil {
		return 0, err
	}
	return len(b), nil
}
