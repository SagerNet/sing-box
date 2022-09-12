package vless

import (
	"encoding/binary"
	"io"
	"net"
	"os"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

type XUDPConn struct {
	net.Conn
	key            []byte
	destination    M.Socksaddr
	requestWritten bool
	responseRead   bool
}

func (c *XUDPConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	return 0, nil, os.ErrInvalid
}

func (c *XUDPConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	start := buffer.Start()
	if !c.responseRead {
		err = ReadResponse(c.Conn)
		if err != nil {
			return
		}
		c.responseRead = true
		buffer.FullReset()
	}
	_, err = buffer.ReadFullFrom(c.Conn, 6)
	if err != nil {
		return
	}
	var length uint16
	err = binary.Read(buffer, binary.BigEndian, &length)
	if err != nil {
		return
	}
	header, err := buffer.ReadBytes(4)
	if err != nil {
		return
	}
	switch header[2] {
	case 1:
		// frame new
		return M.Socksaddr{}, E.New("unexpected frame new")
	case 2:
		// frame keep
		if length != 4 {
			_, err = buffer.ReadFullFrom(c.Conn, int(length)-2)
			if err != nil {
				return
			}
			buffer.Advance(1)
			destination, err = AddressSerializer.ReadAddrPort(buffer)
			if err != nil {
				return
			}
		} else {
			_, err = buffer.ReadFullFrom(c.Conn, 2)
			if err != nil {
				return
			}
			destination = c.destination
		}
	case 3:
		// frame end
		return M.Socksaddr{}, io.EOF
	case 4:
		// frame keep alive
	default:
		return M.Socksaddr{}, E.New("unexpected frame: ", buffer.Byte(2))
	}
	// option error
	if header[3]&2 == 2 {
		return M.Socksaddr{}, E.Cause(net.ErrClosed, "remote closed")
	}
	// option data
	if header[3]&1 != 1 {
		buffer.Resize(start, 0)
		return c.ReadPacket(buffer)
	} else {
		err = binary.Read(buffer, binary.BigEndian, &length)
		if err != nil {
			return
		}
		buffer.Resize(start, 0)
		_, err = buffer.ReadFullFrom(c.Conn, int(length))
		return
	}
}

func (c *XUDPConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	destination := M.SocksaddrFromNet(addr)
	headerLen := c.frontHeadroom(AddressSerializer.AddrPortLen(destination))
	buffer := buf.NewSize(headerLen + len(p))
	buffer.Advance(headerLen)
	common.Must1(buffer.Write(p))
	err = c.WritePacket(buffer, destination)
	if err == nil {
		n = len(p)
	}
	return
}

func (c *XUDPConn) frontHeadroom(addrLen int) int {
	if !c.requestWritten {
		var headerLen int
		headerLen += 1  // version
		headerLen += 16 // uuid
		headerLen += 1  // protobuf length
		headerLen += 1  // command
		headerLen += 2  // frame len
		headerLen += 5  // frame header
		headerLen += addrLen
		headerLen += 2 // payload len
		return headerLen
	} else {
		return 7 + addrLen + 2
	}
}

func (c *XUDPConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	dataLen := buffer.Len()
	addrLen := M.SocksaddrSerializer.AddrPortLen(destination)
	if !c.requestWritten {
		header := buf.With(buffer.ExtendHeader(c.frontHeadroom(addrLen)))
		common.Must(
			header.WriteByte(Version),
			common.Error(header.Write(c.key)),
			header.WriteByte(0),
			header.WriteByte(CommandMux),
			binary.Write(header, binary.BigEndian, uint16(5+addrLen)),
			header.WriteByte(0),
			header.WriteByte(0),
			header.WriteByte(1), // frame type new
			header.WriteByte(1), // option data
			header.WriteByte(NetworkUDP),
			AddressSerializer.WriteAddrPort(header, destination),
			binary.Write(header, binary.BigEndian, uint16(dataLen)),
		)
		c.requestWritten = true
	} else {
		header := buffer.ExtendHeader(c.frontHeadroom(addrLen))
		binary.BigEndian.PutUint16(header, uint16(5+addrLen))
		header[2] = 0
		header[3] = 0
		header[4] = 2 // frame keep
		header[5] = 1 // option data
		header[6] = NetworkUDP
		err := AddressSerializer.WriteAddrPort(buf.With(header[7:]), destination)
		if err != nil {
			return err
		}
		binary.BigEndian.PutUint16(header[7+addrLen:], uint16(dataLen))
	}
	return common.Error(c.Conn.Write(buffer.Bytes()))
}

func (c *XUDPConn) FrontHeadroom() int {
	return c.frontHeadroom(M.MaxSocksaddrLength)
}

func (c *XUDPConn) Upstream() any {
	return c.Conn
}
