package v2raywebsocket

import (
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/websocket"
)

type WebsocketConn struct {
	*websocket.Conn
	*Writer
	remoteAddr net.Addr
	reader     io.Reader
}

func NewServerConn(wsConn *websocket.Conn, remoteAddr net.Addr) *WebsocketConn {
	return &WebsocketConn{
		Conn:       wsConn,
		remoteAddr: remoteAddr,
		Writer:     NewWriter(wsConn, true),
	}
}

func (c *WebsocketConn) Close() error {
	err := c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(C.TCPTimeout))
	if err != nil {
		return c.Conn.Close()
	}
	return nil
}

func (c *WebsocketConn) Read(b []byte) (n int, err error) {
	for {
		if c.reader == nil {
			_, c.reader, err = c.NextReader()
			if err != nil {
				err = wrapError(err)
				return
			}
		}
		n, err = c.reader.Read(b)
		if E.IsMulti(err, io.EOF) {
			c.reader = nil
			continue
		}
		err = wrapError(err)
		return
	}
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

func (c *WebsocketConn) SetReadDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *WebsocketConn) SetWriteDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *WebsocketConn) NeedAdditionalReadDeadline() bool {
	return true
}

func (c *WebsocketConn) Upstream() any {
	return c.Conn.NetConn()
}

func (c *WebsocketConn) UpstreamWriter() any {
	return c.Writer
}

type EarlyWebsocketConn struct {
	*Client
	ctx    context.Context
	conn   *WebsocketConn
	create chan struct{}
	err    error
}

func (c *EarlyWebsocketConn) Read(b []byte) (n int, err error) {
	if c.conn == nil {
		<-c.create
		if c.err != nil {
			return 0, c.err
		}
	}
	return c.conn.Read(b)
}

func (c *EarlyWebsocketConn) writeRequest(content []byte) error {
	var (
		earlyData []byte
		lateData  []byte
		conn      *websocket.Conn
		response  *http.Response
		err       error
	)
	if len(content) > int(c.maxEarlyData) {
		earlyData = content[:c.maxEarlyData]
		lateData = content[c.maxEarlyData:]
	} else {
		earlyData = content
	}
	if len(earlyData) > 0 {
		earlyDataString := base64.RawURLEncoding.EncodeToString(earlyData)
		if c.earlyDataHeaderName == "" {
			requestURL := c.requestURL
			requestURL.Path += earlyDataString
			conn, response, err = c.dialer.DialContext(c.ctx, requestURL.String(), c.headers)
		} else {
			headers := c.headers.Clone()
			headers.Set(c.earlyDataHeaderName, earlyDataString)
			conn, response, err = c.dialer.DialContext(c.ctx, c.requestURLString, headers)
		}
	} else {
		conn, response, err = c.dialer.DialContext(c.ctx, c.requestURLString, c.headers)
	}
	if err != nil {
		return wrapDialError(response, err)
	}
	c.conn = &WebsocketConn{Conn: conn, Writer: NewWriter(conn, false)}
	if len(lateData) > 0 {
		_, err = c.conn.Write(lateData)
	}
	return err
}

func (c *EarlyWebsocketConn) Write(b []byte) (n int, err error) {
	if c.conn != nil {
		return c.conn.Write(b)
	}
	err = c.writeRequest(b)
	c.err = err
	close(c.create)
	if err != nil {
		return
	}
	return len(b), nil
}

func (c *EarlyWebsocketConn) WriteBuffer(buffer *buf.Buffer) error {
	if c.conn != nil {
		return c.conn.WriteBuffer(buffer)
	}
	err := c.writeRequest(buffer.Bytes())
	c.err = err
	close(c.create)
	return err
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
	return os.ErrInvalid
}

func (c *EarlyWebsocketConn) SetReadDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *EarlyWebsocketConn) SetWriteDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *EarlyWebsocketConn) NeedAdditionalReadDeadline() bool {
	return true
}

func (c *EarlyWebsocketConn) Upstream() any {
	return common.PtrOrNil(c.conn)
}

func (c *EarlyWebsocketConn) LazyHeadroom() bool {
	return c.conn == nil
}

func wrapError(err error) error {
	if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		return io.EOF
	}
	if websocket.IsCloseError(err, websocket.CloseAbnormalClosure) {
		return net.ErrClosed
	}
	return err
}
