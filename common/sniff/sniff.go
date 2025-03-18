package sniff

import (
	"bytes"
	"context"
	goerrors "errors"
	"io"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
)

type (
	StreamSniffer = func(ctx context.Context, metadata *adapter.InboundContext, reader io.Reader) error
	PacketSniffer = func(ctx context.Context, metadata *adapter.InboundContext, packet []byte) error
)

func Skip(metadata *adapter.InboundContext) bool {
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

func PeekStream(ctx context.Context, metadata *adapter.InboundContext, conn net.Conn, buffers []*buf.Buffer, buffer *buf.Buffer, timeout time.Duration, sniffers ...StreamSniffer) error {
	if timeout == 0 {
		timeout = C.ReadPayloadTimeout
	}
	deadline := time.Now().Add(timeout)
	var errors []error
	for {
		err := conn.SetReadDeadline(deadline)
		if err != nil {
			return E.Cause(err, "set read deadline")
		}
		_, err = buffer.ReadOnceFrom(conn)
		_ = conn.SetReadDeadline(time.Time{})
		if err != nil {
			return E.Cause(err, "read payload")
		}
		errors = nil
		for _, sniffer := range sniffers {
			reader := io.MultiReader(common.Map(append(buffers, buffer), func(it *buf.Buffer) io.Reader {
				return bytes.NewReader(it.Bytes())
			})...)
			err = sniffer(ctx, metadata, reader)
			if err == nil {
				return nil
			}
			errors = append(errors, err)
		}
		if !goerrors.Is(E.Errors(errors...), errPossibleClientHello) {
			break
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
