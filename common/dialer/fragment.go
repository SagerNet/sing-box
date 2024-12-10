package dialer

import (
	"io"
	"net"
	"os"
	"time"

	opts "github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
)

type TLSFragment struct {
	Enabled bool
	Sleep   IntRange
	Size    IntRange
}

type fragmentConn struct {
	conn        net.Conn
	err         error
	dialer      net.Dialer
	destination M.Socksaddr
	network     string
	fragment    TLSFragment
}

type IntRange struct {
	Min uint64
	Max uint64
}

// isClientHelloPacket checks if data resembles a TLS clientHello packet
func isClientHelloPacket(b []byte) bool {
	// Check if the packet is at least 5 bytes long and the content type is 22 (TLS handshake)
	if len(b) < 5 || b[0] != 22 {
		return false
	}

	// Check if the protocol version is TLS 1.0 or higher (0x0301 or greater)
	version := uint16(b[1])<<8 | uint16(b[2])
	if version < 0x0301 {
		return false
	}

	// Check if the handshake message type is ClientHello (1)
	if b[5] != 1 {
		return false
	}

	return true
}

func (c *fragmentConn) writeFragments(b []byte) (n int, err error) {
	recordLen := 5 + ((int(b[3]) << 8) | int(b[4]))
	if len(b) < recordLen { // maybe already fragmented somehow
		return c.conn.Write(b)
	}

	var bytesWritten int
	data := b[5:recordLen]
	buf := make([]byte, 1024)
	queue := make([]byte, 2048)
	n_queue := int(opts.GetRandomIntFromRange(1, 4))
	L_queue := 0
	c_queue := 0
	for from := 0; ; {
		to := from + int(opts.GetRandomIntFromRange(c.fragment.Size.Min, c.fragment.Size.Max))
		if to > len(data) {
			to = len(data)
		}
		copy(buf[:3], b)
		copy(buf[5:], data[from:to])
		l := to - from
		from = to
		buf[3] = byte(l >> 8)
		buf[4] = byte(l)

		if c_queue < n_queue {
			if l > 0 {
				copy(queue[L_queue:], buf[:5+l])
				L_queue = L_queue + 5 + l
			}
			c_queue = c_queue + 1
		} else {
			if l > 0 {
				copy(queue[L_queue:], buf[:5+l])
				L_queue = L_queue + 5 + l
			}

			if L_queue > 0 {
				n, err := c.conn.Write(queue[:L_queue])
				if err != nil {
					return 0, err
				}
				bytesWritten += n
				if c.fragment.Sleep.Max != 0 {
					time.Sleep(time.Duration(opts.GetRandomIntFromRange(c.fragment.Sleep.Min, c.fragment.Sleep.Max)) * time.Millisecond)
				}

			}

			L_queue = 0
			c_queue = 0

		}

		if from == len(data) {
			if L_queue > 0 {
				n, err := c.conn.Write(queue[:L_queue])
				if err != nil {
					return 0, err
				}
				bytesWritten += n
				if c.fragment.Sleep.Max != 0 {
					time.Sleep(time.Duration(opts.GetRandomIntFromRange(c.fragment.Sleep.Min, c.fragment.Sleep.Max)) * time.Millisecond)
				}

			}
			if len(b) > recordLen {
				n, err := c.conn.Write(b[recordLen:])
				if err != nil {
					return recordLen + n, err
				}
				bytesWritten += n
			}
			return bytesWritten, nil
		}
	}
}

func (c *fragmentConn) Write(b []byte) (n int, err error) {
	if c.conn == nil {
		return 0, c.err
	}

	if isClientHelloPacket(b) {
		return c.writeFragments(b)
	}

	return c.conn.Write(b)
}

func (c *fragmentConn) Read(b []byte) (n int, err error) {
	if c.conn == nil {
		return 0, c.err
	}
	return c.conn.Read(b)
}

func (c *fragmentConn) Close() error {
	return common.Close(c.conn)
}

func (c *fragmentConn) LocalAddr() net.Addr {
	if c.conn == nil {
		return M.Socksaddr{}
	}
	return c.conn.LocalAddr()
}

func (c *fragmentConn) RemoteAddr() net.Addr {
	if c.conn == nil {
		return M.Socksaddr{}
	}
	return c.conn.RemoteAddr()
}

func (c *fragmentConn) SetDeadline(t time.Time) error {
	if c.conn == nil {
		return os.ErrInvalid
	}
	return c.conn.SetDeadline(t)
}

func (c *fragmentConn) SetReadDeadline(t time.Time) error {
	if c.conn == nil {
		return os.ErrInvalid
	}
	return c.conn.SetReadDeadline(t)
}

func (c *fragmentConn) SetWriteDeadline(t time.Time) error {
	if c.conn == nil {
		return os.ErrInvalid
	}
	return c.conn.SetWriteDeadline(t)
}

func (c *fragmentConn) Upstream() any {
	return c.conn
}

func (c *fragmentConn) ReaderReplaceable() bool {
	return c.conn != nil
}

func (c *fragmentConn) WriterReplaceable() bool {
	return c.conn != nil
}

func (c *fragmentConn) LazyHeadroom() bool {
	return c.conn == nil
}

func (c *fragmentConn) NeedHandshake() bool {
	return c.conn == nil
}

func (c *fragmentConn) WriteTo(w io.Writer) (n int64, err error) {
	if c.conn == nil {
		return 0, c.err
	}
	return bufio.Copy(w, c.conn)
}
