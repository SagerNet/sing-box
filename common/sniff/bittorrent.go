package sniff

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
)

const (
	trackerConnectFlag    = 0
	trackerProtocolID     = 0x41727101980
	trackerConnectMinSize = 16
)

// BitTorrent detects if the stream is a BitTorrent connection.
// For the BitTorrent protocol specification, see https://www.bittorrent.org/beps/bep_0003.html
func BitTorrent(_ context.Context, metadata *adapter.InboundContext, reader io.Reader) error {
	var first byte
	err := binary.Read(reader, binary.BigEndian, &first)
	if err != nil {
		return E.Cause1(ErrNeedMoreData, err)
	}

	if first != 19 {
		return os.ErrInvalid
	}

	const header = "BitTorrent protocol"
	var protocol [19]byte
	var n int
	n, err = reader.Read(protocol[:])
	if string(protocol[:n]) != header[:n] {
		return os.ErrInvalid
	}
	if err != nil {
		return E.Cause1(ErrNeedMoreData, err)
	}
	if n < 19 {
		return ErrNeedMoreData
	}

	metadata.Protocol = C.ProtocolBitTorrent
	return nil
}

// UTP detects if the packet is a uTP connection packet.
// For the uTP protocol specification, see
//  1. https://www.bittorrent.org/beps/bep_0029.html
//  2. https://github.com/bittorrent/libutp/blob/2b364cbb0650bdab64a5de2abb4518f9f228ec44/utp_internal.cpp#L112
func UTP(_ context.Context, metadata *adapter.InboundContext, packet []byte) error {
	// A valid uTP packet must be at least 20 bytes long.
	if len(packet) < 20 {
		return os.ErrInvalid
	}

	version := packet[0] & 0x0F
	ty := packet[0] >> 4
	if version != 1 || ty > 4 {
		return os.ErrInvalid
	}

	// Validate the extensions
	extension := packet[1]
	reader := bytes.NewReader(packet[20:])
	for extension != 0 {
		err := binary.Read(reader, binary.BigEndian, &extension)
		if err != nil {
			return err
		}
		if extension > 0x04 {
			return os.ErrInvalid
		}
		var length byte
		err = binary.Read(reader, binary.BigEndian, &length)
		if err != nil {
			return err
		}
		_, err = reader.Seek(int64(length), io.SeekCurrent)
		if err != nil {
			return err
		}
	}
	metadata.Protocol = C.ProtocolBitTorrent
	return nil
}

// UDPTracker detects if the packet is a UDP Tracker Protocol packet.
// For the UDP Tracker Protocol specification, see https://www.bittorrent.org/beps/bep_0015.html
func UDPTracker(_ context.Context, metadata *adapter.InboundContext, packet []byte) error {
	if len(packet) < trackerConnectMinSize {
		return os.ErrInvalid
	}
	if binary.BigEndian.Uint64(packet[:8]) != trackerProtocolID {
		return os.ErrInvalid
	}
	if binary.BigEndian.Uint32(packet[8:12]) != trackerConnectFlag {
		return os.ErrInvalid
	}
	metadata.Protocol = C.ProtocolBitTorrent
	return nil
}
