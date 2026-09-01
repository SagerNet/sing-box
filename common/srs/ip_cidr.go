package srs

import (
	"bufio"
	"encoding/binary"
	"io"
	"net/netip"
	"os"

	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/varbin"
)

func ReadPrefix(reader varbin.Reader) (netip.Prefix, error) {
	addrLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return netip.Prefix{}, err
	}
	if addrLen != 4 && addrLen != 16 {
		return netip.Prefix{}, os.ErrInvalid
	}
	var addrBytes [16]byte
	_, err = io.ReadFull(reader, addrBytes[:addrLen])
	if err != nil {
		return netip.Prefix{}, err
	}
	prefixBits, err := reader.ReadByte()
	if err != nil {
		return netip.Prefix{}, err
	}
	return netip.PrefixFrom(M.AddrFromIP(addrBytes[:addrLen]), int(prefixBits)), nil
}

func WritePrefix(writer varbin.Writer, prefix netip.Prefix) error {
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

func WriteRouteAddressSet(writer io.Writer, include []netip.Prefix, exclude []netip.Prefix) error {
	buffered := bufio.NewWriter(writer)
	for _, prefixes := range [][]netip.Prefix{include, exclude} {
		_, err := varbin.WriteUvarint(buffered, uint64(len(prefixes)))
		if err != nil {
			return err
		}
		for _, prefix := range prefixes {
			err = WritePrefix(buffered, prefix)
			if err != nil {
				return err
			}
		}
	}
	return buffered.Flush()
}

func ReadRouteAddressSet(reader io.Reader) ([]netip.Prefix, []netip.Prefix, error) {
	buffered := bufio.NewReader(reader)
	var sets [2][]netip.Prefix
	for index := range sets {
		count, err := binary.ReadUvarint(buffered)
		if err != nil {
			return nil, nil, err
		}
		prefixes := make([]netip.Prefix, 0, min(count, 4096))
		for range count {
			prefix, prefixErr := ReadPrefix(buffered)
			if prefixErr != nil {
				return nil, nil, prefixErr
			}
			prefixes = append(prefixes, prefix)
		}
		sets[index] = prefixes
	}
	return sets[0], sets[1], nil
}
