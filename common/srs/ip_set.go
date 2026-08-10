package srs

import (
	"encoding/binary"
	"io"
	"net/netip"
	"os"
	"unsafe"

	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/varbin"

	"go4.org/netipx"
)

type myIPSet struct {
	rr []myIPRange
}

type myIPRange struct {
	from netip.Addr
	to   netip.Addr
}

func readIPSet(reader varbin.Reader) (*netipx.IPSet, error) {
	version, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if version != 1 {
		return nil, os.ErrInvalid
	}
	// WTF why using uint64 here
	var length uint64
	err = binary.Read(reader, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}
	mySet := &myIPSet{}
	for range length {
		var from, to netip.Addr
		from, err = readIPSetAddr(reader)
		if err != nil {
			return nil, err
		}
		to, err = readIPSetAddr(reader)
		if err != nil {
			return nil, err
		}
		mySet.rr = append(mySet.rr, myIPRange{from: from, to: to})
	}
	return (*netipx.IPSet)(unsafe.Pointer(mySet)), nil
}

func readIPSetAddr(reader varbin.Reader) (netip.Addr, error) {
	addrLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return netip.Addr{}, err
	}
	if addrLen != 4 && addrLen != 16 {
		return netip.Addr{}, os.ErrInvalid
	}
	var addrBytes [16]byte
	_, err = io.ReadFull(reader, addrBytes[:addrLen])
	if err != nil {
		return netip.Addr{}, err
	}
	return M.AddrFromIP(addrBytes[:addrLen]), nil
}

func writeIPSet(writer varbin.Writer, set *netipx.IPSet) error {
	err := writer.WriteByte(1)
	if err != nil {
		return err
	}
	mySet := (*myIPSet)(unsafe.Pointer(set))
	err = binary.Write(writer, binary.BigEndian, uint64(len(mySet.rr)))
	if err != nil {
		return err
	}
	for _, rr := range mySet.rr {
		fromBytes := rr.from.AsSlice()
		_, err = varbin.WriteUvarint(writer, uint64(len(fromBytes)))
		if err != nil {
			return err
		}
		_, err = writer.Write(fromBytes)
		if err != nil {
			return err
		}
		toBytes := rr.to.AsSlice()
		_, err = varbin.WriteUvarint(writer, uint64(len(toBytes)))
		if err != nil {
			return err
		}
		_, err = writer.Write(toBytes)
		if err != nil {
			return err
		}
	}
	return nil
}
