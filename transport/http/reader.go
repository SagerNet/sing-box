package http

import (
	std_bufio "bufio"
	"io"
	"net"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
)

var errHeaderTooLarge = E.New("request header too large")

type Reader struct {
	*std_bufio.Reader
	limiter *readLimiter
}

func NewReader(conn net.Conn) *Reader {
	limiter := &readLimiter{reader: conn, remaining: -1}
	return &Reader{
		Reader:  std_bufio.NewReader(limiter),
		limiter: limiter,
	}
}

func (r *Reader) setLimit(limit int64) {
	r.limiter.remaining = limit
}

func (r *Reader) cachedConn(conn net.Conn) net.Conn {
	if r.Buffered() == 0 {
		return conn
	}
	buffer := buf.NewSize(r.Buffered())
	_, err := buffer.ReadFullFrom(r, buffer.FreeLen())
	if err != nil {
		buffer.Release()
		return conn
	}
	return bufio.NewCachedConn(conn, buffer)
}

type readLimiter struct {
	reader    io.Reader
	remaining int64
}

func (r *readLimiter) Read(p []byte) (int, error) {
	if r.remaining < 0 {
		return r.reader.Read(p)
	}
	if r.remaining == 0 {
		return 0, errHeaderTooLarge
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.reader.Read(p)
	r.remaining -= int64(n)
	return n, err
}

var _ io.Reader = (*readLimiter)(nil)
