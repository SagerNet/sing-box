package v2raywebsocket

import (
	"net"
	"time"
)

type deadConn struct {
	net.Conn
}

func (c *deadConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *deadConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *deadConn) SetWriteDeadline(t time.Time) error {
	return nil
}
