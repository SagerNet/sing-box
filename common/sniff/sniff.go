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

func Skip(metadata adapter.InboundContext) bool {
	// skip server first protocols
	switch metadata.Destination.Port {
	case 25, 465, 587:
		// SMTP
		return true
	case 143, 993:
		// IMAP
		return true
	case 110, 995:
		// POP3
		return true
	}
	return false
}

func PeekStream(ctx context.Context, conn net.Conn, buffer *buf.Buffer, timeout time.Duration, sniffers ...StreamSniffer) (*adapter.InboundContext, error) {
	if timeout == 0 {
		timeout = C.ReadPayloadTimeout
	}
	deadline := time.Now().Add(timeout)
	var errors []error
	err := conn.SetReadDeadline(deadline)
	if err != nil {
		return nil, E.Cause(err, "set read deadline")
	}
	defer conn.SetReadDeadline(time.Time{})
	var metadata *adapter.InboundContext
	for _, sniffer := range sniffers {
		if buffer.IsEmpty() {
			metadata, err = sniffer(ctx, io.TeeReader(conn, buffer))
		} else {
			metadata, err = sniffer(ctx, io.MultiReader(bytes.NewReader(buffer.Bytes()), io.TeeReader(conn, buffer)))
		}
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
