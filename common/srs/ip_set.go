package srs

import (
	"encoding/binary"
	"io"
	"net/netip"
	"unsafe"

	"github.com/sagernet/sing/common/rw"

	"go4.org/netipx"
)

type myIPSet struct {
	rr []myIPRange
}

type myIPRange struct {
	from netip.Addr
	to   netip.Addr
}

func readIPSet(reader io.Reader) (*netipx.IPSet, error) {
	var version uint8
	err := binary.Read(reader, binary.BigEndian, &version)
	if err != nil {
		return nil, err
	}
	var length uint64
	err = binary.Read(reader, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}
	mySet := &myIPSet{
		rr: make([]myIPRange, length),
	}
	for i := uint64(0); i < length; i++ {
		var (
			fromLen  uint64
			toLen    uint64
			fromAddr netip.Addr
			toAddr   netip.Addr
		)
		fromLen, err = rw.ReadUVariant(reader)
		if err != nil {
			return nil, err
		}
		fromBytes := make([]byte, fromLen)
		_, err = io.ReadFull(reader, fromBytes)
		if err != nil {
			return nil, err
		}
		err = fromAddr.UnmarshalBinary(fromBytes)
		if err != nil {
			return nil, err
		}
		toLen, err = rw.ReadUVariant(reader)
		if err != nil {
			return nil, err
		}
		toBytes := make([]byte, toLen)
		_, err = io.ReadFull(reader, toBytes)
		if err != nil {
			return nil, err
		}
		err = toAddr.UnmarshalBinary(toBytes)
		if err != nil {
			return nil, err
		}
		mySet.rr[i] = myIPRange{fromAddr, toAddr}
	}
	return (*netipx.IPSet)(unsafe.Pointer(mySet)), nil
}

func writeIPSet(writer io.Writer, set *netipx.IPSet) error {
	err := binary.Write(writer, binary.BigEndian, uint8(1))
	if err != nil {
		return err
	}
	mySet := (*myIPSet)(unsafe.Pointer(set))
	err = binary.Write(writer, binary.BigEndian, uint64(len(mySet.rr)))
	if err != nil {
		return err
	}
	for _, rr := range mySet.rr {
		var (
			fromBinary []byte
			toBinary   []byte
		)
		fromBinary, err = rr.from.MarshalBinary()
		if err != nil {
			return err
		}
		err = rw.WriteUVariant(writer, uint64(len(fromBinary)))
		if err != nil {
			return err
		}
		_, err = writer.Write(fromBinary)
		if err != nil {
			return err
		}
		toBinary, err = rr.to.MarshalBinary()
		if err != nil {
			return err
		}
		err = rw.WriteUVariant(writer, uint64(len(toBinary)))
		if err != nil {
			return err
		}
		_, err = writer.Write(toBinary)
		if err != nil {
			return err
		}
	}
	return nil
}
