package sniff

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
)

type (
	StreamSniffer = func(ctx context.Context, reader io.Reader) (*adapter.InboundContext, error)
	PacketSniffer = func(ctx context.Context, packet []byte) (*adapter.InboundContext, error)
)

func PeekStream(ctx context.Context, conn net.Conn, buffer *buf.Buffer, sniffers ...StreamSniffer) (*adapter.InboundContext, error) {
	err := conn.SetReadDeadline(time.Now().Add(C.ReadPayloadTimeout))
	if err != nil {
		return nil, err
	}
	_, err = buffer.ReadOnceFrom(conn)
	err = E.Errors(err, conn.SetReadDeadline(time.Time{}))
	if err != nil {
		return nil, err
	}
	var metadata *adapter.InboundContext
	for _, sniffer := range sniffers {
		metadata, err = sniffer(ctx, bytes.NewReader(buffer.Bytes()))
		if err != nil {
			continue
		}
		return metadata, nil
	}
	return nil, os.ErrInvalid
}

func PeekPacket(ctx context.Context, packet []byte, sniffers ...PacketSniffer) (*adapter.InboundContext, error) {
	for _, sniffer := range sniffers {
		sniffMetadata, err := sniffer(ctx, packet)
		if err != nil {
			continue
		}
		return sniffMetadata, nil
	}
	return nil, os.ErrInvalid
}
