package inbound

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
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
	version         int
	password        string
	fallbackAfter   int
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
	inbound.version = options.Version
	switch options.Version {
	case 0:
		fallthrough
	case 1:
	case 2:
		if options.FallbackAfter == nil {
			inbound.fallbackAfter = 2
		} else {
			inbound.fallbackAfter = *options.FallbackAfter
		}
	case 3:
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
	switch s.version {
	case 1:
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
	case 2:
		hashConn := shadowtls.NewHashWriteConn(conn, s.password)
		go bufio.Copy(hashConn, handshakeConn)
		var request *buf.Buffer
		request, err = s.copyUntilHandshakeFinishedV2(ctx, handshakeConn, conn, hashConn, s.fallbackAfter)
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
	default:
		fallthrough
	case 3:
		var clientHelloFrame *buf.Buffer
		clientHelloFrame, err = shadowtls.ExtractFrame(conn)
		if err != nil {
			return E.Cause(err, "read client handshake")
		}
		_, err = handshakeConn.Write(clientHelloFrame.Bytes())
		if err != nil {
			clientHelloFrame.Release()
			return E.Cause(err, "write client handshake")
		}
		err = shadowtls.VerifyClientHello(clientHelloFrame.Bytes(), s.password)
		if err != nil {
			s.logger.WarnContext(ctx, E.Cause(err, "client hello verify failed"))
			return bufio.CopyConn(ctx, conn, handshakeConn)
		}
		s.logger.TraceContext(ctx, "client hello verify success")
		clientHelloFrame.Release()

		var serverHelloFrame *buf.Buffer
		serverHelloFrame, err = shadowtls.ExtractFrame(handshakeConn)
		if err != nil {
			return E.Cause(err, "read server handshake")
		}

		_, err = conn.Write(serverHelloFrame.Bytes())
		if err != nil {
			serverHelloFrame.Release()
			return E.Cause(err, "write server handshake")
		}

		serverRandom := shadowtls.ExtractServerRandom(serverHelloFrame.Bytes())

		if serverRandom == nil {
			s.logger.WarnContext(ctx, "server random extract failed, will copy bidirectional")
			return bufio.CopyConn(ctx, conn, handshakeConn)
		}

		if !shadowtls.IsServerHelloSupportTLS13(serverHelloFrame.Bytes()) {
			s.logger.WarnContext(ctx, "TLS 1.3 is not supported, will copy bidirectional")
			return bufio.CopyConn(ctx, conn, handshakeConn)
		}

		serverHelloFrame.Release()
		s.logger.TraceContext(ctx, "client authenticated. server random extracted: ", hex.EncodeToString(serverRandom))

		hmacWrite := hmac.New(sha1.New, []byte(s.password))
		hmacWrite.Write(serverRandom)

		hmacAdd := hmac.New(sha1.New, []byte(s.password))
		hmacAdd.Write(serverRandom)
		hmacAdd.Write([]byte("S"))

		hmacVerify := hmac.New(sha1.New, []byte(s.password))
		hmacVerifyReset := func() {
			hmacVerify.Reset()
			hmacVerify.Write(serverRandom)
			hmacVerify.Write([]byte("C"))
		}

		var clientFirstFrame *buf.Buffer
		var group task.Group
		var handshakeFinished bool
		group.Append("client handshake relay", func(ctx context.Context) error {
			clientFrame, cErr := shadowtls.CopyByFrameUntilHMACMatches(conn, handshakeConn, hmacVerify, hmacVerifyReset)
			if cErr == nil {
				clientFirstFrame = clientFrame
				handshakeFinished = true
				handshakeConn.Close()
			}
			return cErr
		})
		group.Append("server handshake relay", func(ctx context.Context) error {
			cErr := shadowtls.CopyByFrameWithModification(handshakeConn, conn, s.password, serverRandom, hmacWrite)
			if E.IsClosedOrCanceled(cErr) && handshakeFinished {
				return nil
			}
			return cErr
		})
		group.Cleanup(func() {
			handshakeConn.Close()
		})
		err = group.Run(ctx)
		if err != nil {
			return E.Cause(err, "handshake relay")
		}

		s.logger.TraceContext(ctx, "handshake relay finished")
		return s.newConnection(ctx, bufio.NewCachedConn(shadowtls.NewVerifiedConn(conn, hmacAdd, hmacVerify, nil), clientFirstFrame), metadata)
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

func (s *ShadowTLS) copyUntilHandshakeFinishedV2(ctx context.Context, dst net.Conn, src io.Reader, hash *shadowtls.HashWriteConn, fallbackAfter int) (*buf.Buffer, error) {
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
			if hash.HasContent() && length >= 8 {
				checksum := hash.Sum()
				if bytes.Equal(data.To(8), checksum) {
					s.logger.TraceContext(ctx, "match current hashcode")
					data.Advance(8)
					return data, nil
				} else if hash.LastSum() != nil && bytes.Equal(data.To(8), hash.LastSum()) {
					s.logger.TraceContext(ctx, "match last hashcode")
					data.Advance(8)
					return data, nil
				}
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
		if applicationDataCount > fallbackAfter {
			return nil, os.ErrPermission
		}
	}
}
