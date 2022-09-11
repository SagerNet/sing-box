package v2rayhttp

import (
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/sagernet/sing-box/common/baderror"
	"github.com/sagernet/sing/common"
)

type HTTPConn struct {
	reader io.Reader
	writer io.Writer
	create chan struct{}
	err    error
}

func newHTTPConn(reader io.Reader, writer io.Writer) HTTPConn {
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
	return os.ErrInvalid
}

func (c *HTTPConn) SetReadDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *HTTPConn) SetWriteDeadline(t time.Time) error {
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
