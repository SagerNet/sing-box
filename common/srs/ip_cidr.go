package srs

import (
	"encoding/binary"
	"net/netip"

	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/varbin"
)

func readPrefix(reader varbin.Reader) (netip.Prefix, error) {
	addrSlice, err := varbin.ReadValue[[]byte](reader, binary.BigEndian)
	if err != nil {
		return netip.Prefix{}, err
	}
	prefixBits, err := varbin.ReadValue[uint8](reader, binary.BigEndian)
	if err != nil {
		return netip.Prefix{}, err
	}
	return netip.PrefixFrom(M.AddrFromIP(addrSlice), int(prefixBits)), nil
}

func writePrefix(writer varbin.Writer, prefix netip.Prefix) error {
	err := varbin.Write(writer, binary.BigEndian, prefix.Addr().AsSlice())
	if err != nil {
		return err
	}
	err = binary.Write(writer, binary.BigEndian, uint8(prefix.Bits()))
	if err != nil {
		return err
	}
	return nil
}
