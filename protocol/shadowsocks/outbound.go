package shadowsocks

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/mux"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/sip003"
	"github.com/sagernet/sing-shadowsocks2"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.ShadowsocksOutboundOptions](registry, C.TypeShadowsocks, NewOutbound)
}

type Outbound struct {
	outbound.Adapter
	logger          logger.ContextLogger
	dialer          N.Dialer
	method          shadowsocks.Method
	serverAddr      M.Socksaddr
	plugin          sip003.Plugin
	uotClient       *uot.Client
	multiplexDialer *mux.Client
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.ShadowsocksOutboundOptions) (adapter.Outbound, error) {
	method, err := shadowsocks.CreateMethod(ctx, options.Method, shadowsocks.MethodOptions{
		Password: options.Password,
	})
	if err != nil {
		return nil, err
	}
	outboundDialer, err := dialer.New(ctx, options.DialerOptions, options.ServerIsDomain())
	if err != nil {
		return nil, err
	}
	outbound := &Outbound{
		Adapter:    outbound.NewAdapterWithDialerOptions(C.TypeShadowsocks, tag, options.Network.Build(), options.DialerOptions),
		logger:     logger,
		dialer:     outboundDialer,
		method:     method,
		serverAddr: options.ServerOptions.Build(),
	}
	if options.Plugin != "" {
		outbound.plugin, err = sip003.CreatePlugin(ctx, options.Plugin, options.PluginOptions, router, outbound.dialer, outbound.serverAddr)
		if err != nil {
			return nil, err
		}
	}
	uotOptions := common.PtrValueOrDefault(options.UDPOverTCP)
	if !uotOptions.Enabled {
		outbound.multiplexDialer, err = mux.NewClientWithOptions((*shadowsocksDialer)(outbound), logger, common.PtrValueOrDefault(options.Multiplex))
		if err != nil {
			return nil, err
		}
	}
	if uotOptions.Enabled {
		outbound.uotClient = &uot.Client{
			Dialer:  (*shadowsocksDialer)(outbound),
			Version: uotOptions.Version,
		}
	}
	return outbound, nil
}

func (h *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination
	if h.multiplexDialer == nil {
		switch N.NetworkName(network) {
		case N.NetworkTCP:
			h.logger.InfoContext(ctx, "outbound connection to ", destination)
		case N.NetworkUDP:
			if h.uotClient != nil {
				h.logger.InfoContext(ctx, "outbound UoT connect packet connection to ", destination)
				return h.uotClient.DialContext(ctx, network, destination)
			} else {
				h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
			}
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

func (h *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination
	if h.multiplexDialer == nil {
		if h.uotClient != nil {
			h.logger.InfoContext(ctx, "outbound UoT packet connection to ", destination)
			return h.uotClient.ListenPacket(ctx, destination)
		} else {
			h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
		}
		h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
		return (*shadowsocksDialer)(h).ListenPacket(ctx, destination)
	} else {
		h.logger.InfoContext(ctx, "outbound multiplex packet connection to ", destination)
		return h.multiplexDialer.ListenPacket(ctx, destination)
	}
}

func (h *Outbound) InterfaceUpdated() {
	if h.multiplexDialer != nil {
		h.multiplexDialer.Reset()
	}
}

func (h *Outbound) Close() error {
	return common.Close(common.PtrOrNil(h.multiplexDialer))
}

var _ N.Dialer = (*shadowsocksDialer)(nil)

type shadowsocksDialer Outbound

func (h *shadowsocksDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination
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
		return bufio.NewBindPacketConn(h.method.DialPacketConn(outConn), destination), nil
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
}

func (h *shadowsocksDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination
	outConn, err := h.dialer.DialContext(ctx, N.NetworkUDP, h.serverAddr)
	if err != nil {
		return nil, err
	}
	return h.method.DialPacketConn(outConn), nil
}
