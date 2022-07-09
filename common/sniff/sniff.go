package sniff

import (
	"context"
	"io"
	"os"

	"github.com/sagernet/sing-box/adapter"
)

type (
	StreamSniffer = func(ctx context.Context, reader io.Reader) (*adapter.InboundContext, error)
	PacketSniffer = func(ctx context.Context, packet []byte) (*adapter.InboundContext, error)
)

func PeekStream(ctx context.Context, reader io.Reader, sniffers ...StreamSniffer) (*adapter.InboundContext, error) {
	for _, sniffer := range sniffers {
		sniffMetadata, err := sniffer(ctx, reader)
		if err != nil {
			return nil, err
		}
		return sniffMetadata, nil
	}
	return nil, os.ErrInvalid
}

func PeekPacket(ctx context.Context, packet []byte, sniffers ...PacketSniffer) (*adapter.InboundContext, error) {
	for _, sniffer := range sniffers {
		sniffMetadata, err := sniffer(ctx, packet)
		if err != nil {
			println(err.Error())
			return nil, err
		}
		return sniffMetadata, nil
	}
	return nil, os.ErrInvalid
}
