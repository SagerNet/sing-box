package sniff

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"io/ioutil"
	"math"
	"os"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
)

func BittorrentTCPMessage(ctx context.Context, reader io.Reader) (*adapter.InboundContext, error) {
	packet, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, os.ErrInvalid
	}
	if len(packet) < 20 {
		return nil, os.ErrInvalid
	}
	if packet[0] != 19 || string(packet[1:20]) != "BitTorrent protocol" {
		return nil, os.ErrInvalid
	}
	return &adapter.InboundContext{Protocol: C.ProtocolBittorrent}, nil
}

func BittorrentUDPMessage(ctx context.Context, packet []byte) (*adapter.InboundContext, error) {
	pLen := len(packet)
	if pLen < 20 {
		return nil, os.ErrInvalid
	}

	buffer := bytes.NewReader(packet)

	var typeAndVersion uint8

	if binary.Read(buffer, binary.BigEndian, &typeAndVersion) != nil {
		return nil, os.ErrInvalid
	} else if packet[0]>>4&0xF > 4 || packet[0]&0xF != 1 {
		return nil, os.ErrInvalid
	}

	var extension uint8

	if binary.Read(buffer, binary.BigEndian, &extension) != nil {
		return nil, os.ErrInvalid
	} else if extension != 0 && extension != 1 {
		return nil, os.ErrInvalid
	}

	for extension != 0 {
		if extension != 1 {
			return nil, os.ErrInvalid
		}
		if binary.Read(buffer, binary.BigEndian, &extension) != nil {
			return nil, os.ErrInvalid
		}

		var length uint8

		if err := binary.Read(buffer, binary.BigEndian, &length); err != nil {
			return nil, os.ErrInvalid
		}
		if int32(pLen) >= int32(length) {
			return nil, os.ErrInvalid
		}
	}

	if int32(pLen) >= int32(2) {
		return nil, os.ErrInvalid
	}

	var timestamp uint32

	if err := binary.Read(buffer, binary.BigEndian, &timestamp); err != nil {
		return nil, os.ErrInvalid
	}
	if math.Abs(float64(time.Now().UnixMicro()-int64(timestamp))) > float64(24*time.Hour) {
		return nil, os.ErrInvalid
	}
	return &adapter.InboundContext{Protocol: C.ProtocolBittorrent}, nil
}
