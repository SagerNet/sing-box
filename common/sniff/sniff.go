package sniff

import (
	"bytes"
	"context"
	"io"
	"net"
	"time"

	"github.com/jobberrt/sing-box/adapter"
	C "github.com/jobberrt/sing-box/constant"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
)

type (
	StreamSniffer = func(ctx context.Context, reader io.Reader) (*adapter.InboundContext, error)
	PacketSniffer = func(ctx context.Context, packet []byte) (*adapter.InboundContext, error)
)

func PeekStream(ctx context.Context, conn net.Conn, buffer *buf.Buffer, timeout time.Duration, sniffers ...StreamSniffer) (*adapter.InboundContext, error) {
	if timeout == 0 {
		timeout = C.ReadPayloadTimeout
	}
	err := conn.SetReadDeadline(time.Now().Add(timeout))
	if err != nil {
		return nil, err
	}
	_, err = buffer.ReadOnceFrom(conn)
	err = E.Errors(err, conn.SetReadDeadline(time.Time{}))
	if err != nil {
		return nil, err
	}
	var metadata *adapter.InboundContext
	var errors []error
	for _, sniffer := range sniffers {
		metadata, err = sniffer(ctx, bytes.NewReader(buffer.Bytes()))
		if metadata != nil {
			return metadata, nil
		}
		errors = append(errors, err)
	}
	return nil, E.Errors(errors...)
}

func PeekPacket(ctx context.Context, packet []byte, sniffers ...PacketSniffer) (*adapter.InboundContext, error) {
	var errors []error
	for _, sniffer := range sniffers {
		metadata, err := sniffer(ctx, packet)
		if metadata != nil {
			return metadata, nil
		}
		errors = append(errors, err)
	}
	return nil, E.Errors(errors...)
}
