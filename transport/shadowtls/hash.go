package shadowtls

import (
	"crypto/hmac"
	"crypto/sha1"
	"hash"
	"net"
)

type HashReadConn struct {
	net.Conn
	hmac hash.Hash
}

func NewHashReadConn(conn net.Conn, password string) *HashReadConn {
	return &HashReadConn{
		conn,
		hmac.New(sha1.New, []byte(password)),
	}
}

func (c *HashReadConn) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	if err != nil {
		return
	}
	_, err = c.hmac.Write(b[:n])
	return
}

func (c *HashReadConn) Sum() []byte {
	return c.hmac.Sum(nil)[:8]
}

type HashWriteConn struct {
	net.Conn
	hmac hash.Hash
}

func NewHashWriteConn(conn net.Conn, password string) *HashWriteConn {
	return &HashWriteConn{
		conn,
		hmac.New(sha1.New, []byte(password)),
	}
}

func (c *HashWriteConn) Write(p []byte) (n int, err error) {
	if c.hmac != nil {
		c.hmac.Write(p)
	}
	return c.Conn.Write(p)
}

func (c *HashWriteConn) Sum() []byte {
	return c.hmac.Sum(nil)[:8]
}

func (c *HashWriteConn) Fallback() {
	c.hmac = nil
}
