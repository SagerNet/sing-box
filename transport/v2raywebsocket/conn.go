package v2raywebsocket

import (
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/gorilla/websocket"
)

type WebsocketConn struct {
	*websocket.Conn
	remoteAddr net.Addr
	reader     io.Reader
}

func (c *WebsocketConn) Read(b []byte) (n int, err error) {
	for {
		if c.reader == nil {
			_, c.reader, err = c.NextReader()
			if err != nil {
				return
			}
		}
		n, err = c.reader.Read(b)
		if E.IsMulti(err, io.EOF) {
			c.reader = nil
			continue
		}
		return
	}
}

func (c *WebsocketConn) Write(b []byte) (n int, err error) {
	err = c.WriteMessage(websocket.BinaryMessage, b)
	if err != nil {
		return
	}
	return len(b), nil
}

func (c *WebsocketConn) RemoteAddr() net.Addr {
	if c.remoteAddr != nil {
		return c.remoteAddr
	}
	return c.Conn.RemoteAddr()
}

func (c *WebsocketConn) SetDeadline(t time.Time) error {
	return os.ErrInvalid
}

type EarlyWebsocketConn struct {
	*Client
	conn   *WebsocketConn
	create chan struct{}
}

func (c *EarlyWebsocketConn) Read(b []byte) (n int, err error) {
	if c.conn == nil {
		<-c.create
	}
	return c.conn.Read(b)
}

func (c *EarlyWebsocketConn) Write(b []byte) (n int, err error) {
	if c.conn != nil {
		return c.conn.Write(b)
	}
	var (
		earlyData []byte
		lateData  []byte
		conn      *websocket.Conn
		response  *http.Response
	)
	if len(earlyData) > int(c.maxEarlyData) {
		earlyData = earlyData[:c.maxEarlyData]
		lateData = lateData[c.maxEarlyData:]
	} else {
		earlyData = b
	}
	if len(earlyData) > 0 {
		earlyDataString := base64.RawURLEncoding.EncodeToString(earlyData)
		if c.earlyDataHeaderName == "" {
			conn, response, err = c.dialer.Dial(c.uri+earlyDataString, c.headers)
		} else {
			headers := c.headers.Clone()
			headers.Set(c.earlyDataHeaderName, earlyDataString)
			conn, response, err = c.dialer.Dial(c.uri, headers)
		}
	} else {
		conn, response, err = c.dialer.Dial(c.uri, c.headers)
	}
	if err != nil {
		return 0, wrapDialError(response, err)
	}
	c.conn = &WebsocketConn{Conn: conn}
	close(c.create)
	if len(lateData) > 0 {
		_, err = c.conn.Write(lateData)
	}
	if err != nil {
		return
	}
	return len(b), nil
}

func (c *EarlyWebsocketConn) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *EarlyWebsocketConn) LocalAddr() net.Addr {
	if c.conn == nil {
		return nil
	}
	return c.conn.LocalAddr()
}

func (c *EarlyWebsocketConn) RemoteAddr() net.Addr {
	if c.conn == nil {
		return nil
	}
	return c.conn.RemoteAddr()
}

func (c *EarlyWebsocketConn) SetDeadline(t time.Time) error {
	if c.conn == nil {
		return os.ErrInvalid
	}
	return c.conn.SetDeadline(t)
}

func (c *EarlyWebsocketConn) SetReadDeadline(t time.Time) error {
	if c.conn == nil {
		return os.ErrInvalid
	}
	return c.conn.SetReadDeadline(t)
}

func (c *EarlyWebsocketConn) SetWriteDeadline(t time.Time) error {
	if c.conn == nil {
		return os.ErrInvalid
	}
	return c.conn.SetWriteDeadline(t)
}
