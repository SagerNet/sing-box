//go:build with_shadowsocksr

package outbound

import (
	"context"
	"net"
	"strings"

	"github.com/sagernet/shadowsocksr"
	"github.com/sagernet/shadowsocksr/obfs"
	"github.com/sagernet/shadowsocksr/protocol"
	"github.com/sagernet/shadowsocksr/ssr"
	"github.com/sagernet/shadowsocksr/streamCipher"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-shadowsocks"
	"github.com/sagernet/sing-shadowsocks/shadowimpl"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Outbound = (*ShadowsocksR)(nil)

type ShadowsocksR struct {
	myOutboundAdapter
	dialer         N.Dialer
	serverAddr     M.Socksaddr
	method         shadowsocks.Method
	cipher         string
	password       string
	obfs           string
	obfsParams     *ssr.ServerInfo
	protocol       string
	protocolParams *ssr.ServerInfo
}

func NewShadowsocksR(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowsocksROutboundOptions) (*ShadowsocksR, error) {
	outbound := &ShadowsocksR{
		myOutboundAdapter: myOutboundAdapter{
			protocol: C.TypeShadowsocksR,
			network:  options.Network.Build(),
			router:   router,
			logger:   logger,
			tag:      tag,
		},
		dialer:     dialer.New(router, options.DialerOptions),
		serverAddr: options.ServerOptions.Build(),
		cipher:     options.Method,
		password:   options.Password,
		obfs:       options.Obfs,
		protocol:   options.Protocol,
	}
	var err error
	outbound.method, err = shadowimpl.FetchMethod(options.Method, options.Password)
	if err != nil {
		return nil, err
	}
	if _, err = streamCipher.NewStreamCipher(options.Method, options.Password); err != nil {
		return nil, E.New(strings.ToLower(err.Error()))
	}
	if obfs.NewObfs(options.Obfs) == nil {
		return nil, E.New("unknown obfs: " + options.Obfs)
	}
	outbound.obfsParams = &ssr.ServerInfo{
		Host:   outbound.serverAddr.AddrString(),
		Port:   outbound.serverAddr.Port,
		TcpMss: 1460,
		Param:  options.ObfsParam,
	}
	if protocol.NewProtocol(options.Protocol) == nil {
		return nil, E.New("unknown protocol: " + options.Protocol)
	}
	outbound.protocolParams = &ssr.ServerInfo{
		Host:   outbound.serverAddr.AddrString(),
		Port:   outbound.serverAddr.Port,
		TcpMss: 1460,
		Param:  options.Protocol,
	}
	if outbound.method == nil {
		outbound.network = common.Filter(outbound.network, func(it string) bool { return it == N.NetworkTCP })
	}
	return outbound, nil
}

func (h *ShadowsocksR) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch network {
	case N.NetworkTCP:
		h.logger.InfoContext(ctx, "outbound connection to ", destination)
		conn, err := h.dialer.DialContext(ctx, network, h.serverAddr)
		if err != nil {
			return nil, err
		}
		cipher, err := streamCipher.NewStreamCipher(h.cipher, h.password)
		if err != nil {
			return nil, E.New(strings.ToLower(err.Error()))
		}
		ssConn := shadowsocksr.NewSSTCPConn(conn, cipher)
		ssConn.IObfs = obfs.NewObfs(h.obfs)
		ssConn.IObfs.SetServerInfo(h.obfsParams)
		ssConn.IProtocol = protocol.NewProtocol(h.protocol)
		ssConn.IProtocol.SetServerInfo(h.protocolParams)
		err = M.SocksaddrSerializer.WriteAddrPort(ssConn, destination)
		if err != nil {
			return nil, E.Cause(err, "write request")
		}
		return ssConn, nil
	case N.NetworkUDP:
		conn, err := h.ListenPacket(ctx, destination)
		if err != nil {
			return nil, err
		}
		return &bufio.BindPacketConn{PacketConn: conn, Addr: destination}, nil
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
}

func (h *ShadowsocksR) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	outConn, err := h.dialer.DialContext(ctx, N.NetworkUDP, h.serverAddr)
	if err != nil {
		return nil, err
	}
	return h.method.DialPacketConn(outConn), nil
}

func (h *ShadowsocksR) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewConnection(ctx, h, conn, metadata)
}

func (h *ShadowsocksR) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return NewPacketConnection(ctx, h, conn, metadata)
}
