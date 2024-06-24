package srs

import (
	"encoding/binary"
	"net/netip"
	"os"
	"unsafe"

	"github.com/sagernet/sing/common"
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

type myIPRangeData struct {
	From []byte
	To   []byte
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
	ranges := make([]myIPRangeData, length)
	err = varbin.Read(reader, binary.BigEndian, &ranges)
	if err != nil {
		return nil, err
	}
	mySet := &myIPSet{
		rr: make([]myIPRange, len(ranges)),
	}
	for i, rangeData := range ranges {
		mySet.rr[i].from = M.AddrFromIP(rangeData.From)
		mySet.rr[i].to = M.AddrFromIP(rangeData.To)
	}
	return (*netipx.IPSet)(unsafe.Pointer(mySet)), nil
}

func writeIPSet(writer varbin.Writer, set *netipx.IPSet) error {
	err := writer.WriteByte(1)
	if err != nil {
		return err
	}
	dataList := common.Map((*myIPSet)(unsafe.Pointer(set)).rr, func(rr myIPRange) myIPRangeData {
		return myIPRangeData{
			From: rr.from.AsSlice(),
			To:   rr.to.AsSlice(),
		}
	})
	err = binary.Write(writer, binary.BigEndian, uint64(len(dataList)))
	if err != nil {
		return err
	}
	for _, data := range dataList {
		err = varbin.Write(writer, binary.BigEndian, data)
		if err != nil {
			return err
		}
	}
	return nil
}
