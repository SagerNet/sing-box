package outbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/mux"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/sip003"
	"github.com/sagernet/sing-shadowsocks"
	"github.com/sagernet/sing-shadowsocks/shadowimpl"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"
)

var _ adapter.Outbound = (*Shadowsocks)(nil)

type Shadowsocks struct {
	myOutboundAdapter
	dialer          N.Dialer
	method          shadowsocks.Method
	serverAddr      M.Socksaddr
	plugin          sip003.Plugin
	uot             bool
	uotVersion      int
	multiplexDialer N.Dialer
}

func NewShadowsocks(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowsocksOutboundOptions) (*Shadowsocks, error) {
	method, err := shadowimpl.FetchMethod(options.Method, options.Password, router.TimeFunc())
	if err != nil {
		return nil, err
	}
	outbound := &Shadowsocks{
		myOutboundAdapter: myOutboundAdapter{
			protocol: C.TypeShadowsocks,
			network:  options.Network.Build(),
			router:   router,
			logger:   logger,
			tag:      tag,
		},
		dialer:     dialer.New(router, options.DialerOptions),
		method:     method,
		serverAddr: options.ServerOptions.Build(),
		uot:        options.UoT,
	}
	if options.Plugin != "" {
		outbound.plugin, err = sip003.CreatePlugin(options.Plugin, options.PluginOptions, router, outbound.dialer, outbound.serverAddr)
		if err != nil {
			return nil, err
		}
	}
	if !options.UoT {
		outbound.multiplexDialer, err = mux.NewClientWithOptions(ctx, (*shadowsocksDialer)(outbound), common.PtrValueOrDefault(options.MultiplexOptions))
		if err != nil {
			return nil, err
		}
	}
	switch options.UoTVersion {
	case uot.LegacyVersion:
		outbound.uotVersion = uot.LegacyVersion
	case 0, uot.Version:
		outbound.uotVersion = uot.Version
	default:
		return nil, E.New("unknown udp over tcp protocol version ", options.UoTVersion)
	}
	return outbound, nil
}

func (h *Shadowsocks) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.AppendContext(ctx)
	metadata.Outbound = h.tag
	metadata.Destination = destination
	if h.multiplexDialer == nil {
		switch N.NetworkName(network) {
		case N.NetworkTCP:
			h.logger.InfoContext(ctx, "outbound connection to ", destination)
		case N.NetworkUDP:
			if h.uot {
				h.logger.InfoContext(ctx, "outbound UoT packet connection to ", destination)
				var uotDestination M.Socksaddr
				if h.uotVersion == uot.Version {
					uotDestination.Fqdn = uot.MagicAddress
				} else {
					uotDestination.Fqdn = uot.LegacyMagicAddress
				}
				tcpConn, err := (*shadowsocksDialer)(h).DialContext(ctx, N.NetworkTCP, uotDestination)
				if err != nil {
					return nil, err
				}
				if h.uotVersion == uot.Version {
					return uot.NewLazyConn(tcpConn, uot.Request{IsConnect: true, Destination: destination}), nil
				} else {
					return uot.NewConn(tcpConn, false, destination), nil
				}
			}
			h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
		}
		return (*shadowsocksDialer)(h).DialContext(ctx, network, destination)
	} else {
		switch N.NetworkName(network) {
		case N.NetworkTCP:
			h.logger.InfoContext(ctx, "outbound multiplex connection to ", destination)
		case N.NetworkUDP:
			h.logger.InfoContext(ctx, "outbound multiplex packet connection to ", destination)
		}
		return h.multiplexDialer.DialContext(ctx, network, destination)
	}
}

func (h *Shadowsocks) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	ctx, metadata := adapter.AppendContext(ctx)
	metadata.Outbound = h.tag
	metadata.Destination = destination
	if h.multiplexDialer == nil {
		if h.uot {
			h.logger.InfoContext(ctx, "outbound UoT packet connection to ", destination)
			var uotDestination M.Socksaddr
			if h.uotVersion == uot.Version {
				uotDestination.Fqdn = uot.MagicAddress
			} else {
				uotDestination.Fqdn = uot.LegacyMagicAddress
			}
			tcpConn, err := (*shadowsocksDialer)(h).DialContext(ctx, N.NetworkTCP, uotDestination)
			if err != nil {
				return nil, err
			}
			if h.uotVersion == uot.Version {
				return uot.NewLazyConn(tcpConn, uot.Request{Destination: destination}), nil
			} else {
				return uot.NewConn(tcpConn, false, destination), nil
			}
		}
		h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
		return (*shadowsocksDialer)(h).ListenPacket(ctx, destination)
	} else {
		h.logger.InfoContext(ctx, "outbound multiplex packet connection to ", destination)
		return h.multiplexDialer.ListenPacket(ctx, destination)
	}
}

func (h *Shadowsocks) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewConnection(ctx, h, conn, metadata)
}

func (h *Shadowsocks) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return NewPacketConnection(ctx, h, conn, metadata)
}

func (h *Shadowsocks) Close() error {
	return common.Close(h.multiplexDialer)
}

var _ N.Dialer = (*shadowsocksDialer)(nil)

type shadowsocksDialer Shadowsocks

func (h *shadowsocksDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		var outConn net.Conn
		var err error
		if h.plugin != nil {
			outConn, err = h.plugin.DialContext(ctx)
		} else {
			outConn, err = h.dialer.DialContext(ctx, N.NetworkTCP, h.serverAddr)
		}
		if err != nil {
			return nil, err
		}
		return h.method.DialEarlyConn(outConn, destination), nil
	case N.NetworkUDP:
		outConn, err := h.dialer.DialContext(ctx, N.NetworkUDP, h.serverAddr)
		if err != nil {
			return nil, err
		}
		return &bufio.BindPacketConn{PacketConn: h.method.DialPacketConn(outConn), Addr: destination}, nil
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
}

func (h *shadowsocksDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	outConn, err := h.dialer.DialContext(ctx, N.NetworkUDP, h.serverAddr)
	if err != nil {
		return nil, err
	}
	return h.method.DialPacketConn(outConn), nil
}
