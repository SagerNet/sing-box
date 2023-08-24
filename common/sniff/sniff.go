package sniff

import (
	"bytes"
	"context"
	"io"
	"net"
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

func PeekStream(ctx context.Context, conn net.Conn, buffer *buf.Buffer, timeout time.Duration, sniffers ...StreamSniffer) (*adapter.InboundContext, error) {
	if timeout == 0 {
		timeout = C.ReadPayloadTimeout
	}
	deadline := time.Now().Add(timeout)
	for i := 0; ; i++ {
		err := conn.SetReadDeadline(deadline)
		if err != nil {
			return nil, E.Cause(err, "set read deadline")
		}
		_, err = buffer.ReadOnceFrom(conn)
		err = E.Errors(err, conn.SetReadDeadline(time.Time{}))
		if i > 0 {
			break
		}
		if err != nil {
			return nil, E.Cause(err, "read payload")
		}
	}
	var errors []error
	for _, sniffer := range sniffers {
		metadata, err := sniffer(ctx, bytes.NewReader(buffer.Bytes()))
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
