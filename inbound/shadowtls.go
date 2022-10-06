package inbound

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/shadowtls"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/task"
)

type ShadowTLS struct {
	myInboundAdapter
	handshakeDialer N.Dialer
	handshakeAddr   M.Socksaddr
	v2              bool
	password        string
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
		password:        options.Password,
	}
	switch options.Version {
	case 0:
		fallthrough
	case 1:
	case 2:
		inbound.v2 = true
	default:
		return nil, E.New("unknown shadowtls protocol version: ", options.Version)
	}
	inbound.connHandler = inbound
	return inbound, nil
}

func (s *ShadowTLS) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	handshakeConn, err := s.handshakeDialer.DialContext(ctx, N.NetworkTCP, s.handshakeAddr)
	if err != nil {
		return err
	}
	if !s.v2 {
		var handshake task.Group
		handshake.Append("client handshake", func(ctx context.Context) error {
			return s.copyUntilHandshakeFinished(handshakeConn, conn)
		})
		handshake.Append("server handshake", func(ctx context.Context) error {
			return s.copyUntilHandshakeFinished(conn, handshakeConn)
		})
		handshake.FastFail()
		handshake.Cleanup(func() {
			handshakeConn.Close()
		})
		err = handshake.Run(ctx)
		if err != nil {
			return err
		}
		return s.newConnection(ctx, conn, metadata)
	} else {
		hashConn := shadowtls.NewHashWriteConn(conn, s.password)
		go bufio.Copy(hashConn, handshakeConn)
		var request *buf.Buffer
		request, err = s.copyUntilHandshakeFinishedV2(handshakeConn, conn, hashConn)
		if err == nil {
			handshakeConn.Close()
			return s.newConnection(ctx, bufio.NewCachedConn(shadowtls.NewConn(conn), request), metadata)
		} else if err == os.ErrPermission {
			s.logger.WarnContext(ctx, "fallback connection")
			hashConn.Fallback()
			return common.Error(bufio.Copy(handshakeConn, conn))
		} else {
			return err
		}
	}
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

func (s *ShadowTLS) copyUntilHandshakeFinishedV2(dst net.Conn, src io.Reader, hash *shadowtls.HashWriteConn) (*buf.Buffer, error) {
	const applicationData = 0x17
	var tlsHdr [5]byte
	var applicationDataCount int
	for {
		_, err := io.ReadFull(src, tlsHdr[:])
		if err != nil {
			return nil, err
		}
		length := binary.BigEndian.Uint16(tlsHdr[3:])
		if tlsHdr[0] == applicationData {
			data := buf.NewSize(int(length))
			_, err = data.ReadFullFrom(src, int(length))
			if err != nil {
				data.Release()
				return nil, err
			}
			if length >= 8 && bytes.Equal(data.To(8), hash.Sum()) {
				data.Advance(8)
				return data, nil
			}
			_, err = io.Copy(dst, io.MultiReader(bytes.NewReader(tlsHdr[:]), data))
			data.Release()
			applicationDataCount++
		} else {
			_, err = io.Copy(dst, io.MultiReader(bytes.NewReader(tlsHdr[:]), io.LimitReader(src, int64(length))))
		}
		if err != nil {
			return nil, err
		}
		if applicationDataCount > 3 {
			return nil, os.ErrPermission
		}
	}
}
