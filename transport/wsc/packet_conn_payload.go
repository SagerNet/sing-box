package wsc

type packetConnPayload struct {
	ip   [16]byte
	port uint16
}

/*

import (
	"encoding/binary"
	"errors"
	"net"
)

// Header is 18 bytes: 16 for IP + 2 for port (big-endian)
type Header struct {
	IP   [16]byte
	Port uint16
}

const (
	headerLen = 18
)

// ipv4ToMapped fills a 16-byte buffer with ::ffff:w.x.y.z
func ipv4ToMapped(v4 net.IP, dst *[16]byte) {
	// v4 must be 4 bytes (no zone)
	copy(dst[:], []byte{
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0xff, 0xff,
		v4[0], v4[1], v4[2], v4[3],
	})
}

// ipTo16Mapped returns a 16-byte IPv6 form.
// - IPv6 stays as-is (compressed/expanded form doesn't matter; we copy the 16 raw bytes).
// - IPv4 becomes IPv4-mapped IPv6 ::ffff:w.x.y.z
func ipTo16Mapped(ip net.IP) ([16]byte, error) {
	var out [16]byte
	if ip == nil {
		return out, errors.New("nil IP")
	}
	if v4 := ip.To4(); v4 != nil {
		ipv4ToMapped(v4, &out)
		return out, nil
	}
	v6 := ip.To16()
	if v6 == nil || len(v6) != 16 {
		return out, errors.New("invalid IP")
	}
	copy(out[:], v6)
	return out, nil
}

// NewHeader builds a Header from net.IP + port.
func NewHeader(ip net.IP, port int) (Header, error) {
	var h Header
	ip16, err := ipTo16Mapped(ip)
	if err != nil {
		return h, err
	}
	h.IP = ip16
	if port < 0 || port > 65535 {
		return h, errors.New("invalid port")
	}
	h.Port = uint16(port)
	return h, nil
}

// FromTCPAddr / FromUDPAddr convenience.
func FromTCPAddr(a *net.TCPAddr) (Header, error) { return NewHeader(a.IP, a.Port) }
func FromUDPAddr(a *net.UDPAddr) (Header, error) { return NewHeader(a.IP, a.Port) }

// MarshalBinary -> 18 bytes
func (h Header) MarshalBinary() []byte {
	b := make([]byte, headerLen)
	copy(b[:16], h.IP[:])
	binary.BigEndian.PutUint16(b[16:], h.Port)
	return b
}

// UnmarshalBinary <- 18 bytes
func (h *Header) UnmarshalBinary(b []byte) error {
	if len(b) < headerLen {
		return errors.New("short header")
	}
	copy(h.IP[:], b[:16])
	h.Port = binary.BigEndian.Uint16(b[16:18])
	return nil
}

// ToNetAddr returns a *net.TCPAddr or *net.UDPAddr-ready IP & port.
// If the address is IPv4-mapped, it returns the 4-byte form for convenience.
func (h Header) ToIPPort() (net.IP, int) {
	ip := net.IP(h.IP[:]).To16()
	// Detect IPv4-mapped ::ffff:w.x.y.z and convert back to v4 if you like:
	if ip4 := ip.To4(); ip4 != nil {
		return ip4, int(h.Port)
	}
	return ip, int(h.Port)
}
*/

/*
// Encode
dst := net.ParseIP("192.0.2.10")
hdr, _ := NewHeader(dst, 443)
wireBytes := hdr.MarshalBinary() // 18 bytes ready to send

// Decode
var got Header
_ = got.UnmarshalBinary(wireBytes)
ip, port := got.ToIPPort() // ip is 4-byte 192.0.2.10, port=443
*/
