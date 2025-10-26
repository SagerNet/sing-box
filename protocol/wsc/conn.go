package wsc

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/sagernet/ws"
	"github.com/sagernet/ws/wsutil"
)

type wsStreamConn struct {
	conn   net.Conn
	reader *wsutil.Reader
	writer *wsutil.Writer
	mutex  sync.Mutex
	closed bool
}

func newWSStreamConn(conn net.Conn) net.Conn {
	return &wsStreamConn{
		conn: conn,
		reader: &wsutil.Reader{
			Source: conn,
			State:  ws.StateClientSide,
		},
		writer: wsutil.NewWriter(conn, ws.StateClientSide, ws.OpBinary),
	}
}

func (wsConn *wsStreamConn) Read(payload []byte) (int, error) {
	for {
		hdr, err := wsConn.reader.NextFrame()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0, io.EOF
			}
			return 0, err
		}

		if hdr.OpCode.IsControl() {
			continue
		}

		if hdr.OpCode == ws.OpText {
			if _, err := io.Copy(io.Discard, wsConn.reader); err != nil {
				return 0, err
			}
			continue
		}

		if hdr.OpCode == ws.OpBinary {
			return wsConn.reader.Read(payload)
		}

		return 0, nil
	}
}

func (wsConn *wsStreamConn) Write(payload []byte) (int, error) {
	wsConn.mutex.Lock()
	defer wsConn.mutex.Unlock()

	wsConn.writer.Reset(wsConn.conn, ws.StateClientSide, ws.OpBinary)

	if _, err := wsConn.writer.Write(payload); err != nil {
		return 0, err
	}
	if err := wsConn.writer.Flush(); err != nil {
		return 0, err
	}
	return len(payload), nil
}

func (wsConn *wsStreamConn) Close() error {
	if wsConn.closed {
		return nil
	}
	wsConn.closed = true
	return wsConn.conn.Close()
}

func (wsConn *wsStreamConn) LocalAddr() net.Addr {
	return wsConn.conn.LocalAddr()
}

func (wsConn *wsStreamConn) RemoteAddr() net.Addr {
	return wsConn.conn.RemoteAddr()
}

func (wsConn *wsStreamConn) SetDeadline(dead time.Time) error {
	return wsConn.conn.SetDeadline(dead)
}

func (wsConn *wsStreamConn) SetReadDeadline(dead time.Time) error {
	return wsConn.conn.SetReadDeadline(dead)
}

func (wsConn *wsStreamConn) SetWriteDeadline(dead time.Time) error {
	return wsConn.conn.SetWriteDeadline(dead)
}
