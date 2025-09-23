package v2raywebsocket

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/debug"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/ws"
	"github.com/sagernet/ws/wsutil"
)

type WebsocketConn struct {
	net.Conn
	*Writer
	state          ws.State
	reader         *wsutil.Reader
	controlHandler wsutil.FrameHandlerFunc
	remoteAddr     net.Addr
}

func NewConn(conn net.Conn, remoteAddr net.Addr, state ws.State) *WebsocketConn {
	controlHandler := wsutil.ControlFrameHandler(conn, state)
	return &WebsocketConn{
		Conn:  conn,
		state: state,
		reader: &wsutil.Reader{
			Source:          conn,
			State:           state,
			SkipHeaderCheck: !debug.Enabled,
			OnIntermediate:  controlHandler,
		},
		controlHandler: controlHandler,
		remoteAddr:     remoteAddr,
		Writer:         NewWriter(conn, state),
	}
}

func (c *WebsocketConn) Close() error {
	c.Conn.SetWriteDeadline(time.Now().Add(C.TCPTimeout))
	frame := ws.NewCloseFrame(ws.NewCloseFrameBody(
		ws.StatusNormalClosure, "",
	))
	if c.state == ws.StateClientSide {
		frame = ws.MaskFrameInPlace(frame)
	}
	ws.WriteFrame(c.Conn, frame)
	c.Conn.Close()
	return nil
}

func (c *WebsocketConn) Read(b []byte) (n int, err error) {
	var header ws.Header
	for {
		n, err = c.reader.Read(b)
		if n > 0 {
			err = nil
			return
		}
		if !E.IsMulti(err, io.EOF, wsutil.ErrNoFrameAdvance) {
			err = wrapWsError(err)
			return
		}
		header, err = wrapWsError0(c.reader.NextFrame())
		if err != nil {
			return
		}
		if header.OpCode.IsControl() {
			if header.Length > 128 {
				err = wsutil.ErrFrameTooLarge
				return
			}
			err = wrapWsError(c.controlHandler(header, c.reader))
			if err != nil {
				return
			}
			continue
		}
		if header.OpCode&ws.OpBinary == 0 {
			err = wrapWsError(c.reader.Discard())
			if err != nil {
				return
			}
			continue
		}
	}
}

func (c *WebsocketConn) Write(p []byte) (n int, err error) {
	err = wrapWsError(wsutil.WriteMessage(c.Conn, c.state, ws.OpBinary, p))
	if err != nil {
		return
	}
	n = len(p)
	return
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
	return c.Conn
}

type EarlyWebsocketConn struct {
	*Client
	ctx    context.Context
	conn   atomic.Pointer[WebsocketConn]
	access sync.Mutex
	create chan struct{}
	err    error
}

func (c *EarlyWebsocketConn) Read(b []byte) (n int, err error) {
	conn := c.conn.Load()
	if conn == nil {
		<-c.create
		if c.err != nil {
			return 0, c.err
		}
		conn = c.conn.Load()
	}
	return wrapWsError0(conn.Read(b))
}

func (c *EarlyWebsocketConn) writeRequest(content []byte) error {
	var (
		earlyData []byte
		lateData  []byte
		conn      *WebsocketConn
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
			conn, err = c.dialContext(c.ctx, &requestURL, c.headers)
		} else {
			headers := c.headers.Clone()
			headers.Set(c.earlyDataHeaderName, earlyDataString)
			conn, err = c.dialContext(c.ctx, &c.requestURL, headers)
		}
	} else {
		conn, err = c.dialContext(c.ctx, &c.requestURL, c.headers)
	}
	if err != nil {
		return err
	}
	if len(lateData) > 0 {
		_, err = conn.Write(lateData)
		if err != nil {
			return err
		}
	}
	c.conn.Store(conn)
	return nil
}

func (c *EarlyWebsocketConn) Write(b []byte) (n int, err error) {
	conn := c.conn.Load()
	if conn != nil {
		return wrapWsError0(conn.Write(b))
	}
	c.access.Lock()
	defer c.access.Unlock()
	conn = c.conn.Load()
	if c.err != nil {
		return 0, c.err
	}
	if conn != nil {
		return wrapWsError0(conn.Write(b))
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
	conn := c.conn.Load()
	if conn != nil {
		return wrapWsError(conn.WriteBuffer(buffer))
	}
	c.access.Lock()
	defer c.access.Unlock()
	if c.err != nil {
		return c.err
	}
	conn = c.conn.Load()
	if conn != nil {
		return wrapWsError(conn.WriteBuffer(buffer))
	}
	err := c.writeRequest(buffer.Bytes())
	c.err = err
	close(c.create)
	return err
}

func (c *EarlyWebsocketConn) Close() error {
	conn := c.conn.Load()
	if conn == nil {
		return nil
	}
	return conn.Close()
}

func (c *EarlyWebsocketConn) LocalAddr() net.Addr {
	conn := c.conn.Load()
	if conn == nil {
		return M.Socksaddr{}
	}
	return conn.LocalAddr()
}

func (c *EarlyWebsocketConn) RemoteAddr() net.Addr {
	conn := c.conn.Load()
	if conn == nil {
		return M.Socksaddr{}
	}
	return conn.RemoteAddr()
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
	return common.PtrOrNil(c.conn.Load())
}

func (c *EarlyWebsocketConn) LazyHeadroom() bool {
	return c.conn.Load() == nil
}

func wrapWsError(err error) error {
	if err == nil {
		return nil
	}
	var closedErr wsutil.ClosedError
	if errors.As(err, &closedErr) {
		if closedErr.Code == ws.StatusNormalClosure || closedErr.Code == ws.StatusNoStatusRcvd {
			err = io.EOF
		}
	}
	return err
}

func wrapWsError0[T any](value T, err error) (T, error) {
	if err == nil {
		return value, nil
	}
	return value, wrapWsError(err)
}
