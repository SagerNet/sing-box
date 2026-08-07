package srs

import (
	"encoding/binary"
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
	mySet := &myIPSet{
		rr: make([]myIPRange, 0, min(length, 64)),
	}
	for i := uint64(0); i < length; i++ {
		fromLen, err := binary.ReadUvarint(reader)
		if err != nil {
			return nil, err
		}
		fromBytes, err := readBytes(reader, fromLen)
		if err != nil {
			return nil, err
		}
		toLen, err := binary.ReadUvarint(reader)
		if err != nil {
			return nil, err
		}
		toBytes, err := readBytes(reader, toLen)
		if err != nil {
			return nil, err
		}
		mySet.rr = append(mySet.rr, myIPRange{
			from: M.AddrFromIP(fromBytes),
			to:   M.AddrFromIP(toBytes),
		})
	}
	return (*netipx.IPSet)(unsafe.Pointer(mySet)), nil
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
