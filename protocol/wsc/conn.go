package wsc

import (
	"net"
	"time"

	"github.com/sagernet/ws"
	"github.com/sagernet/ws/wsutil"
)

type wsStreamConn struct {
	conn   net.Conn
	reader *wsutil.Reader
	writer *wsutil.Writer
	server bool
}

func newWSStreamConn(conn net.Conn, server bool) net.Conn {
	var state ws.State

	if server {
		state = ws.StateClientSide
	} else {
		state = ws.StateClientSide
	}

	return &wsStreamConn{
		conn: conn,
		reader: &wsutil.Reader{
			Source: conn,
			State:  state,
		},
		writer: wsutil.NewWriter(conn, state, ws.OpBinary),
		server: server,
	}
}

func (wsConn *wsStreamConn) Read(payload []byte) (int, error) {
	return wsConn.read(payload)
}

func (wsConn *wsStreamConn) Write(payload []byte) (int, error) {
	return wsConn.write(payload)
}

func (wsConn *wsStreamConn) Close() error {
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

func (wsConn *wsStreamConn) write(payload []byte) (int, error) {
	if wsConn.server {
		return len(payload), wsutil.WriteServerMessage(wsConn.conn, ws.OpBinary, payload)
	} else {
		return len(payload), wsutil.WriteClientMessage(wsConn.conn, ws.OpBinary, payload)
	}
}

func (wsConn *wsStreamConn) read(payload []byte) (int, error) {
	if wsConn.server {
		readed, _, err := wsutil.ReadClientData(wsConn.conn)
		if err != nil {
			return 0, err
		}
		return copy(payload, readed), nil
	} else {
		readed, _, err := wsutil.ReadServerData(wsConn.conn)
		if err != nil {
			return 0, err
		}
		return copy(payload, readed), nil
	}
}
