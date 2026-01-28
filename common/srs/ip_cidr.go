package srs

import (
	"encoding/binary"
	"io"
	"net/netip"

	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/varbin"
)

func readPrefix(reader varbin.Reader) (netip.Prefix, error) {
	addrLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return netip.Prefix{}, err
	}
	addrSlice := make([]byte, addrLen)
	_, err = io.ReadFull(reader, addrSlice)
	if err != nil {
		return netip.Prefix{}, err
	}
	prefixBits, err := reader.ReadByte()
	if err != nil {
		return netip.Prefix{}, err
	}
	return netip.PrefixFrom(M.AddrFromIP(addrSlice), int(prefixBits)), nil
}

func writePrefix(writer varbin.Writer, prefix netip.Prefix) error {
	addrSlice := prefix.Addr().AsSlice()
	_, err := varbin.WriteUvarint(writer, uint64(len(addrSlice)))
	if err != nil {
		return err
	}
	_, err = writer.Write(addrSlice)
	if err != nil {
		return err
	}
	err = writer.WriteByte(uint8(prefix.Bits()))
	if err != nil {
		return err
	}
	return nil
}
