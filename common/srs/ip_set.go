package srs

import (
	"encoding/binary"
	"io"
	"os"
	"slices"

	"github.com/sagernet/sing-box/common/ipset"
	"github.com/sagernet/sing/common/varbin"
)

func readIPSet(reader varbin.Reader) (*ipset.Set, error) {
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
	var (
		ranges4 []ipset.Range4
		ranges6 []ipset.Range6
	)
	for range length {
		var from, to [16]byte
		var fromLength, toLength int
		fromLength, err = readIPSetAddr(reader, &from)
		if err != nil {
			return nil, err
		}
		toLength, err = readIPSetAddr(reader, &to)
		if err != nil {
			return nil, err
		}
		if fromLength != toLength {
			return nil, os.ErrInvalid
		}
		if fromLength == 4 {
			ranges4 = append(ranges4, ipset.Range4{
				From: binary.BigEndian.Uint32(from[:4]),
				To:   binary.BigEndian.Uint32(to[:4]),
			})
		} else {
			ranges6 = append(ranges6, ipset.Range6{From: from, To: to})
		}
	}
	slices.SortFunc(ranges4, func(a, b ipset.Range4) int {
		if a.From < b.From {
			return -1
		} else if a.From > b.From {
			return 1
		}
		return 0
	})
	slices.SortFunc(ranges6, func(a, b ipset.Range6) int {
		return slices.Compare(a.From[:], b.From[:])
	})
	return ipset.FromRanges(slices.Clip(ranges4), slices.Clip(ranges6), nil)
}

func readIPSetAddr(reader varbin.Reader, addr *[16]byte) (int, error) {
	addrLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return 0, err
	}
	if addrLen != 4 && addrLen != 16 {
		return 0, os.ErrInvalid
	}
	_, err = io.ReadFull(reader, addr[:addrLen])
	if err != nil {
		return 0, err
	}
	return int(addrLen), nil
}

func writeIPSet(writer varbin.Writer, set *ipset.Set) error {
	err := writer.WriteByte(1)
	if err != nil {
		return err
	}
	ranges4 := set.Ranges4()
	ranges6 := set.Ranges6()
	err = binary.Write(writer, binary.BigEndian, uint64(len(ranges4)+len(ranges6)))
	if err != nil {
		return err
	}
	for _, ipRange := range ranges4 {
		var from, to [4]byte
		binary.BigEndian.PutUint32(from[:], ipRange.From)
		binary.BigEndian.PutUint32(to[:], ipRange.To)
		err = writeIPSetAddr(writer, from[:])
		if err != nil {
			return err
		}
		err = writeIPSetAddr(writer, to[:])
		if err != nil {
			return err
		}
	}
	for _, ipRange := range ranges6 {
		err = writeIPSetAddr(writer, ipRange.From[:])
		if err != nil {
			return err
		}
		err = writeIPSetAddr(writer, ipRange.To[:])
		if err != nil {
			return err
		}
	}
	return nil
}

func writeIPSetAddr(writer varbin.Writer, addr []byte) error {
	_, err := varbin.WriteUvarint(writer, uint64(len(addr)))
	if err != nil {
		return err
	}
	_, err = writer.Write(addr)
	return err
}
