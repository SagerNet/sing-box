package route

import (
	"encoding/binary"
	"encoding/hex"
	"net"
	"net/netip"
	"slices"
	"strings"
)

func extractMACFromDUID(duid []byte) (net.HardwareAddr, bool) {
	if len(duid) < 4 {
		return nil, false
	}
	duidType := binary.BigEndian.Uint16(duid[0:2])
	hwType := binary.BigEndian.Uint16(duid[2:4])
	if hwType != 1 {
		return nil, false
	}
	switch duidType {
	case 1:
		if len(duid) < 14 {
			return nil, false
		}
		return net.HardwareAddr(slices.Clone(duid[8:14])), true
	case 3:
		if len(duid) < 10 {
			return nil, false
		}
		return net.HardwareAddr(slices.Clone(duid[4:10])), true
	}
	return nil, false
}

func extractMACFromEUI64(address netip.Addr) (net.HardwareAddr, bool) {
	if !address.Is6() {
		return nil, false
	}
	b := address.As16()
	if b[11] != 0xff || b[12] != 0xfe {
		return nil, false
	}
	return net.HardwareAddr{b[8] ^ 0x02, b[9], b[10], b[13], b[14], b[15]}, true
}

func parseDUID(s string) ([]byte, error) {
	cleaned := strings.ReplaceAll(s, ":", "")
	return hex.DecodeString(cleaned)
}
