package inbound

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/task"
)

type ShadowTLS struct {
	myInboundAdapter
	handshakeDialer N.Dialer
	handshakeAddr   M.Socksaddr
}

func NewShadowTLS(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowTLSInboundOptions) (*ShadowTLS, error) {
	inbound := &ShadowTLS{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeShadowTLS,
			network:       []string{N.NetworkTCP},
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
		handshakeDialer: dialer.New(router, options.Handshake.DialerOptions),
		handshakeAddr:   options.Handshake.ServerOptions.Build(),
	}
	inbound.connHandler = inbound
	return inbound, nil
}

func (s *ShadowTLS) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	handshakeConn, err := s.handshakeDialer.DialContext(ctx, N.NetworkTCP, s.handshakeAddr)
	if err != nil {
		return err
	}
	var handshake task.Group
	handshake.Append("client handshake", func(ctx context.Context) error {
		return s.copyUntilHandshakeFinished(handshakeConn, conn)
	})
	handshake.Append("server handshake", func(ctx context.Context) error {
		return s.copyUntilHandshakeFinished(conn, handshakeConn)
	})
	handshake.FastFail()
	err = handshake.Run(ctx)
	if err != nil {
		return err
	}
	return s.newConnection(ctx, conn, metadata)
}

func (s *ShadowTLS) copyUntilHandshakeFinished(dst io.Writer, src io.Reader) error {
	const handshake = 0x16
	const changeCipherSpec = 0x14
	var hasSeenChangeCipherSpec bool
	var tlsHdr [5]byte
	for {
		_, err := io.ReadFull(src, tlsHdr[:])
		if err != nil {
			return err
		}
		length := binary.BigEndian.Uint16(tlsHdr[3:])
		_, err = io.Copy(dst, io.MultiReader(bytes.NewReader(tlsHdr[:]), io.LimitReader(src, int64(length))))
		if err != nil {
			return err
		}
		if tlsHdr[0] != handshake {
			if tlsHdr[0] != changeCipherSpec {
				return E.New("unexpected tls frame type: ", tlsHdr[0])
			}
			if !hasSeenChangeCipherSpec {
				hasSeenChangeCipherSpec = true
				continue
			}
		}
		if hasSeenChangeCipherSpec {
			return nil
		}
	}
}
