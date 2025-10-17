//go:build go1.25 && badlinkname

package badtls

import (
	"github.com/sagernet/sing/common/buf"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/tls"
)

var _ N.ReadWaiter = (*ReadWaitConn)(nil)

type ReadWaitConn struct {
	tls.Conn
	rawConn         *RawConn
	readWaitOptions N.ReadWaitOptions
}

func NewReadWaitConn(conn tls.Conn) (tls.Conn, error) {
	if _, isReadWaitConn := conn.(N.ReadWaiter); isReadWaitConn {
		return conn, nil
	}
	rawConn, err := NewRawConn(conn)
	if err != nil {
		return nil, err
	}
	return &ReadWaitConn{
		Conn:    conn,
		rawConn: rawConn,
	}, nil
}

func (c *ReadWaitConn) InitializeReadWaiter(options N.ReadWaitOptions) (needCopy bool) {
	c.readWaitOptions = options
	return false
}

func (c *ReadWaitConn) WaitReadBuffer() (buffer *buf.Buffer, err error) {
	//err = c.HandshakeContext(context.Background())
	//if err != nil {
	//	return
	//}
	c.rawConn.In.Lock()
	defer c.rawConn.In.Unlock()
	for c.rawConn.Input.Len() == 0 {
		err = c.rawConn.ReadRecord()
		if err != nil {
			return
		}
		for c.rawConn.Hand.Len() > 0 {
			err = c.rawConn.HandlePostHandshakeMessage()
			if err != nil {
				return
			}
		}
	}
	buffer = c.readWaitOptions.NewBuffer()
	n, err := c.rawConn.Input.Read(buffer.FreeBytes())
	if err != nil {
		buffer.Release()
		return
	}
	buffer.Truncate(n)

	if n != 0 && c.rawConn.Input.Len() == 0 && c.rawConn.Input.Len() > 0 &&
		// recordType(c.RawInput.Bytes()[0]) == recordTypeAlert {
		c.rawConn.RawInput.Bytes()[0] == 21 {
		_ = c.rawConn.ReadRecord()
		// return n, err // will be io.EOF on closeNotify
	}

	c.readWaitOptions.PostReturn(buffer)
	return
}

func (c *ReadWaitConn) Upstream() any {
	return c.Conn
}

func (c *ReadWaitConn) ReaderReplaceable() bool {
	return true
}
