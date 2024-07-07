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
	StreamSniffer = func(ctx context.Context, metadata *adapter.InboundContext, reader io.Reader) error
	PacketSniffer = func(ctx context.Context, metadata *adapter.InboundContext, packet []byte) error
)

func PeekStream(ctx context.Context, metadata *adapter.InboundContext, conn net.Conn, buffer *buf.Buffer, timeout time.Duration, sniffers ...StreamSniffer) error {
	if timeout == 0 {
		timeout = C.ReadPayloadTimeout
	}
	deadline := time.Now().Add(timeout)
	var errors []error

	for i := 0; i < 3; i++ {
		err := conn.SetReadDeadline(deadline)
		if err != nil {
			return E.Cause(err, "set read deadline")
		}
		_, err = buffer.ReadOnceFrom(conn)
		err = E.Errors(err, conn.SetReadDeadline(time.Time{}))
		if err != nil {
			if i > 0 {
				break
			}
			return E.Cause(err, "read payload")
		}
		for _, sniffer := range sniffers {
			err = sniffer(ctx, metadata, bytes.NewReader(buffer.Bytes()))
			if err == nil {
				return nil
			}
			errors = append(errors, err)
		}
	}
	return E.Errors(errors...)
}

func PeekPacket(ctx context.Context, metadata *adapter.InboundContext, packet []byte, sniffers ...PacketSniffer) error {
	var errors []error
	for _, sniffer := range sniffers {
		err := sniffer(ctx, metadata, packet)
		if err == nil {
			return nil
		}
		errors = append(errors, err)
	}
	return E.Errors(errors...)
}
