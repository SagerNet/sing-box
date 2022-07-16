package sniff

import (
	"context"
	"encoding/binary"
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
)

func STUNMessage(ctx context.Context, packet []byte) (*adapter.InboundContext, error) {
	pLen := len(packet)
	if pLen < 20 {
		return nil, os.ErrInvalid
	}
	if binary.BigEndian.Uint32(packet[4:8]) != 0x2112A442 {
		return nil, os.ErrInvalid
	}
	if len(packet) < 20+int(binary.BigEndian.Uint16(packet[2:4])) {
		return nil, os.ErrInvalid
	}
	return &adapter.InboundContext{Protocol: C.ProtocolSTUN}, nil
}
