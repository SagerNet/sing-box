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
}

func (c *HTTPConn) Read(b []byte) (n int, err error) {
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
