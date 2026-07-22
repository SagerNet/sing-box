package v2rayxhttp

import (
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	M "github.com/sagernet/sing/common/metadata"
)

type responseConn struct {
	reader io.Reader
	writer http.ResponseWriter
	done   <-chan struct{}
	access sync.Mutex
	closed bool
}

func newResponseConn(reader io.Reader, writer http.ResponseWriter, done <-chan struct{}) *responseConn {
	return &responseConn{reader: reader, writer: writer, done: done}
}
func (c *responseConn) Read(buffer []byte) (int, error) { return c.reader.Read(buffer) }
func (c *responseConn) Write(buffer []byte) (int, error) {
	c.access.Lock()
	defer c.access.Unlock()
	if c.closed {
		return 0, net.ErrClosed
	}
	n, err := c.writer.Write(buffer)
	if err == nil {
		if flusher, ok := c.writer.(http.Flusher); ok {
			flusher.Flush()
		}
	}
	return n, err
}
func (c *responseConn) Close() error {
	c.access.Lock()
	defer c.access.Unlock()
	c.closed = true
	return nil
}
func (c *responseConn) LocalAddr() net.Addr              { return M.Socksaddr{} }
func (c *responseConn) RemoteAddr() net.Addr             { return M.Socksaddr{} }
func (c *responseConn) SetDeadline(time.Time) error      { return os.ErrInvalid }
func (c *responseConn) SetReadDeadline(time.Time) error  { return os.ErrInvalid }
func (c *responseConn) SetWriteDeadline(time.Time) error { return os.ErrInvalid }
