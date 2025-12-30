package sudoku

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"
)

const (
	UoTMagicByte  byte = 0xEE
	uotVersion         = 0x01
	maxUoTPayload      = 64 * 1024
)

func WritePreface(w io.Writer) error {
	_, err := w.Write([]byte{UoTMagicByte, uotVersion})
	return err
}

func WriteDatagram(w io.Writer, addr string, payload []byte) error {
	addrBuf, err := EncodeAddress(addr)
	if err != nil {
		return fmt.Errorf("encode address: %w", err)
	}

	if addrLen := len(addrBuf); addrLen == 0 || addrLen > maxUoTPayload {
		return fmt.Errorf("address too long: %d", addrLen)
	}
	if payloadLen := len(payload); payloadLen > maxUoTPayload {
		return fmt.Errorf("payload too large: %d", payloadLen)
	}

	var header [4]byte
	binary.BigEndian.PutUint16(header[:2], uint16(len(addrBuf)))
	binary.BigEndian.PutUint16(header[2:], uint16(len(payload)))

	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	if _, err := w.Write(addrBuf); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}

func ReadDatagram(r io.Reader) (string, []byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return "", nil, err
	}

	addrLen := int(binary.BigEndian.Uint16(header[:2]))
	payloadLen := int(binary.BigEndian.Uint16(header[2:]))

	if addrLen <= 0 || addrLen > maxUoTPayload {
		return "", nil, fmt.Errorf("invalid address length: %d", addrLen)
	}
	if payloadLen < 0 || payloadLen > maxUoTPayload {
		return "", nil, fmt.Errorf("invalid payload length: %d", payloadLen)
	}

	addrBuf := make([]byte, addrLen)
	if _, err := io.ReadFull(r, addrBuf); err != nil {
		return "", nil, err
	}

	addr, err := DecodeAddress(bytes.NewReader(addrBuf))
	if err != nil {
		return "", nil, fmt.Errorf("decode address: %w", err)
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return "", nil, err
	}

	return addr, payload, nil
}

// UoTPacketConn adapts a net.Conn carrying Sudoku UoT frames to net.PacketConn.
type UoTPacketConn struct {
	conn    net.Conn
	writeMu sync.Mutex
}

func NewUoTPacketConn(conn net.Conn) *UoTPacketConn {
	return &UoTPacketConn{conn: conn}
}

func (c *UoTPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	for {
		addrStr, payload, err := ReadDatagram(c.conn)
		if err != nil {
			return 0, nil, err
		}

		if len(payload) > len(p) {
			return 0, nil, io.ErrShortBuffer
		}

		host, port, _ := net.SplitHostPort(addrStr)
		portInt, _ := strconv.ParseUint(port, 10, 16)
		ip, err := netip.ParseAddr(host)
		if err != nil {
			// Domains are not supported in net.UDPAddr, drop.
			continue
		}
		udpAddr := net.UDPAddrFromAddrPort(netip.AddrPortFrom(ip.Unmap(), uint16(portInt)))

		copy(p, payload)
		return len(payload), udpAddr, nil
	}
}

func (c *UoTPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	if addr == nil {
		return 0, errors.New("address is nil")
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := WriteDatagram(c.conn, addr.String(), p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *UoTPacketConn) Close() error {
	return c.conn.Close()
}

func (c *UoTPacketConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *UoTPacketConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *UoTPacketConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *UoTPacketConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

