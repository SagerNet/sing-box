package sniff

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/constant"
)

// BitTorrent detects if the stream is a BitTorrent connection.
// For the BitTorrent protocol specification, see https://www.bittorrent.org/beps/bep_0003.html
func BitTorrent(_ context.Context, reader io.Reader) (*adapter.InboundContext, error) {
	var first byte
	err := binary.Read(reader, binary.BigEndian, &first)
	if err != nil {
		return nil, err
	}

	if first != 19 {
		return nil, os.ErrInvalid
	}

	var protocol [19]byte
	_, err = reader.Read(protocol[:])
	if err != nil {
		return nil, err
	}
	if string(protocol[:]) != "BitTorrent protocol" {
		return nil, os.ErrInvalid
	}

	return &adapter.InboundContext{
		Protocol: constant.ProtocolBitTorrent,
	}, nil
}

// UTP detects if the packet is a uTP connection packet.
// For the uTP protocol specification, see
//  1. https://www.bittorrent.org/beps/bep_0029.html
//  2. https://github.com/bittorrent/libutp/blob/2b364cbb0650bdab64a5de2abb4518f9f228ec44/utp_internal.cpp#L112
func UTP(_ context.Context, packet []byte) (*adapter.InboundContext, error) {
	// A valid uTP packet must be at least 20 bytes long.
	if len(packet) < 20 {
		return nil, os.ErrInvalid
	}

	version := packet[0] & 0x0F
	ty := packet[0] >> 4
	if version != 1 || ty > 4 {
		return nil, os.ErrInvalid
	}

	// Validate the extensions
	extension := packet[1]
	reader := bytes.NewReader(packet[20:])
	for extension != 0 {
		err := binary.Read(reader, binary.BigEndian, &extension)
		if err != nil {
			return nil, err
		}

		var length byte
		err = binary.Read(reader, binary.BigEndian, &length)
		if err != nil {
			return nil, err
		}
		_, err = reader.Seek(int64(length), io.SeekCurrent)
		if err != nil {
			return nil, err
		}
	}

	return &adapter.InboundContext{
		Protocol: constant.ProtocolUTP,
	}, nil
}
