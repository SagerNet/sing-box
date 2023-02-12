package v2rayhttp

import (
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/sagernet/sing-box/common/baderror"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
)

type HTTPConn struct {
	reader io.Reader
	writer io.Writer
	create chan struct{}
	err    error
}

func NewHTTPConn(reader io.Reader, writer io.Writer) HTTPConn {
	return HTTPConn{
		reader: reader,
		writer: writer,
	}
}

func newLateHTTPConn(writer io.Writer) *HTTPConn {
	return &HTTPConn{
		create: make(chan struct{}),
		writer: writer,
	}
}

func (c *HTTPConn) setup(reader io.Reader, err error) {
	c.reader = reader
	c.err = err
	close(c.create)
}

func (c *HTTPConn) Read(b []byte) (n int, err error) {
	if c.reader == nil {
		<-c.create
		if c.err != nil {
			return 0, c.err
		}
	}
	n, err = c.reader.Read(b)
	return n, baderror.WrapH2(err)
}

func (c *HTTPConn) Write(b []byte) (n int, err error) {
	n, err = c.writer.Write(b)
	return n, baderror.WrapH2(err)
}

func (c *HTTPConn) Close() error {
	return common.Close(c.reader, c.writer)
}

func (c *HTTPConn) LocalAddr() net.Addr {
	return nil
}

func (c *HTTPConn) RemoteAddr() net.Addr {
	return nil
}

func (c *HTTPConn) SetDeadline(t time.Time) error {
	if responseWriter, loaded := c.writer.(interface {
		SetWriteDeadline(time.Time) error
	}); loaded {
		return responseWriter.SetWriteDeadline(t)
	}
	return os.ErrInvalid
}

func (c *HTTPConn) SetReadDeadline(t time.Time) error {
	if responseWriter, loaded := c.writer.(interface {
		SetReadDeadline(time.Time) error
	}); loaded {
		return responseWriter.SetReadDeadline(t)
	}
	return os.ErrInvalid
}

func (c *HTTPConn) SetWriteDeadline(t time.Time) error {
	if responseWriter, loaded := c.writer.(interface {
		SetWriteDeadline(time.Time) error
	}); loaded {
		return responseWriter.SetWriteDeadline(t)
	}
	return os.ErrInvalid
}

type ServerHTTPConn struct {
	HTTPConn
	flusher http.Flusher
}

func (c *ServerHTTPConn) Write(b []byte) (n int, err error) {
	n, err = c.writer.Write(b)
	if err == nil {
		c.flusher.Flush()
	}
	return
}

type HTTP2ConnWrapper struct {
	N.ExtendedConn
	access sync.Mutex
	closed bool
}

func NewHTTP2Wrapper(conn net.Conn) *HTTP2ConnWrapper {
	return &HTTP2ConnWrapper{
		ExtendedConn: bufio.NewExtendedConn(conn),
	}
}

func (w *HTTP2ConnWrapper) Write(p []byte) (n int, err error) {
	w.access.Lock()
	defer w.access.Unlock()
	if w.closed {
		return 0, net.ErrClosed
	}
	return w.ExtendedConn.Write(p)
}

func (w *HTTP2ConnWrapper) WriteBuffer(buffer *buf.Buffer) error {
	w.access.Lock()
	defer w.access.Unlock()
	if w.closed {
		return net.ErrClosed
	}
	return w.ExtendedConn.WriteBuffer(buffer)
}

func (w *HTTP2ConnWrapper) CloseWrapper() {
	w.access.Lock()
	defer w.access.Unlock()
	w.closed = true
}

func (w *HTTP2ConnWrapper) Upstream() any {
	return w.ExtendedConn
}
