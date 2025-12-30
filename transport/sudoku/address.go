package sudoku

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
)

// EncodeAddress encodes a "host:port" string into the SOCKS5 address format used by Sudoku.
func EncodeAddress(rawAddr string) ([]byte, error) {
	host, portStr, err := net.SplitHostPort(rawAddr)
	if err != nil {
		return nil, err
	}

	portInt, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, err
	}

	var buf []byte
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			buf = append(buf, 0x01) // IPv4
			buf = append(buf, ip4...)
		} else {
			buf = append(buf, 0x04) // IPv6
			buf = append(buf, ip...)
		}
	} else {
		if len(host) > 255 {
			return nil, fmt.Errorf("domain too long")
		}
		buf = append(buf, 0x03) // domain
		buf = append(buf, byte(len(host)))
		buf = append(buf, host...)
	}

	var portBytes [2]byte
	binary.BigEndian.PutUint16(portBytes[:], uint16(portInt))
	buf = append(buf, portBytes[:]...)
	return buf, nil
}

// DecodeAddress decodes a SOCKS5 address from the given reader and returns it as "host:port".
func DecodeAddress(r io.Reader) (string, error) {
	var atyp [1]byte
	if _, err := io.ReadFull(r, atyp[:]); err != nil {
		return "", err
	}

	switch atyp[0] {
	case 0x01: // IPv4
		var ipBuf [net.IPv4len]byte
		if _, err := io.ReadFull(r, ipBuf[:]); err != nil {
			return "", err
		}
		var portBuf [2]byte
		if _, err := io.ReadFull(r, portBuf[:]); err != nil {
			return "", err
		}
		return net.JoinHostPort(net.IP(ipBuf[:]).String(), fmt.Sprint(binary.BigEndian.Uint16(portBuf[:]))), nil
	case 0x04: // IPv6
		var ipBuf [net.IPv6len]byte
		if _, err := io.ReadFull(r, ipBuf[:]); err != nil {
			return "", err
		}
		var portBuf [2]byte
		if _, err := io.ReadFull(r, portBuf[:]); err != nil {
			return "", err
		}
		return net.JoinHostPort(net.IP(ipBuf[:]).String(), fmt.Sprint(binary.BigEndian.Uint16(portBuf[:]))), nil
	case 0x03: // domain
		var lengthBuf [1]byte
		if _, err := io.ReadFull(r, lengthBuf[:]); err != nil {
			return "", err
		}
		l := int(lengthBuf[0])
		hostBuf := make([]byte, l)
		if _, err := io.ReadFull(r, hostBuf); err != nil {
			return "", err
		}
		var portBuf [2]byte
		if _, err := io.ReadFull(r, portBuf[:]); err != nil {
			return "", err
		}
		return net.JoinHostPort(string(hostBuf), fmt.Sprint(binary.BigEndian.Uint16(portBuf[:]))), nil
	default:
		return "", fmt.Errorf("unknown address type: %d", atyp[0])
	}
}

