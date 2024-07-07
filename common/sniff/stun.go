package sniff

import (
	"context"
	"encoding/binary"
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
)

func STUNMessage(_ context.Context, metadata *adapter.InboundContext, packet []byte) error {
	pLen := len(packet)
	if pLen < 20 {
		return os.ErrInvalid
	}
	if binary.BigEndian.Uint32(packet[4:8]) != 0x2112A442 {
		return os.ErrInvalid
	}
	if len(packet) < 20+int(binary.BigEndian.Uint16(packet[2:4])) {
		return os.ErrInvalid
	}
	metadata.Protocol = C.ProtocolSTUN
	return nil
}
