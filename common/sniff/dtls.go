package sniff

import (
	"context"
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
)

func DTLSRecord(ctx context.Context, metadata *adapter.InboundContext, packet []byte) error {
	const fixedHeaderSize = 13
	if len(packet) < fixedHeaderSize {
		return os.ErrInvalid
	}
	contentType := packet[0]
	switch contentType {
	case 20, 21, 22, 23, 25:
	default:
		return os.ErrInvalid
	}
	versionMajor := packet[1]
	if versionMajor != 0xfe {
		return os.ErrInvalid
	}
	versionMinor := packet[2]
	if versionMinor != 0xff && versionMinor != 0xfd {
		return os.ErrInvalid
	}
	metadata.Protocol = C.ProtocolDTLS
	return nil
}
