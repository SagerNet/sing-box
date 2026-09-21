package http

import (
	"context"
	"net"
	"slices"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/http"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/net/http2"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.HTTPOutboundOptions](registry, C.TypeHTTP, NewOutbound)
}

type Outbound struct {
	outbound.Adapter
	logger logger.ContextLogger
	client *http.Client
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.HTTPOutboundOptions) (adapter.Outbound, error) {
	outboundDialer, err := dialer.New(ctx, options.DialerOptions, options.ServerIsDomain())
	if err != nil {
		return nil, err
	}
	headers := options.Headers.Build()
	version := http.ResolveVersion(options.Version, options.Path, headers.Get("Host"))
	tlsOptions := common.PtrValueOrDefault(options.TLS)
	if version == 3 && !tlsOptions.Enabled {
		return nil, C.ErrTLSRequired
	}
	alpnIsDefault := tlsOptions.Enabled && len(tlsOptions.ALPN) == 0
	if alpnIsDefault {
		switch version {
		case 1:
			tlsOptions.ALPN = []string{"http/1.1"}
		default:
			tlsOptions.ALPN = []string{http2.NextProtoTLS, "http/1.1"}
		}
	}
	detour, err := tls.NewDialerFromOptions(ctx, logger, outboundDialer, options.Server, tlsOptions)
	if err != nil {
		return nil, err
	}
	var http1Detour N.Dialer
	if version >= 2 && (alpnIsDefault || slices.Contains(tlsOptions.ALPN, "http/1.1")) {
		http1TLSOptions := tlsOptions
		http1TLSOptions.ALPN = []string{"http/1.1"}
		http1Detour, err = tls.NewDialerFromOptions(ctx, logger, outboundDialer, options.Server, http1TLSOptions)
		if err != nil {
			return nil, err
		}
	}
	var quicTLSConfig tls.Config
	if version == 3 {
		if alpnIsDefault {
			tlsOptions.ALPN = []string{"h3"}
		}
		quicTLSConfig, err = tls.NewClient(ctx, logger, options.Server, tlsOptions)
		if err != nil {
			return nil, err
		}
	}
	client, err := http.NewClient(http.ClientOptions{
		Dialer:                 detour,
		HTTP1Dialer:            http1Detour,
		RawDialer:              outboundDialer,
		TLSConfig:              quicTLSConfig,
		Server:                 options.ServerOptions.Build(),
		Username:               options.Username,
		Password:               options.Password,
		Path:                   options.Path,
		Headers:                headers,
		Version:                version,
		DisableVersionFallback: options.DisableVersionFallback,
		HTTP2Options:           options.HTTP2Options,
		HTTP3Options:           options.HTTP3Options,
	})
	if err != nil {
		return nil, err
	}
	return &Outbound{
		Adapter: outbound.NewAdapterWithDialerOptions(C.TypeHTTP, tag, []string{N.NetworkTCP, N.NetworkUDP}, options.DialerOptions),
		logger:  logger,
		client:  client,
	}, nil
}

func (h *Outbound) InterfaceUpdated(ctx context.Context) {
	h.client.ResetConnections()
}

func (h *Outbound) Close() error {
	return h.client.Close()
}

func (h *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		h.logger.InfoContext(ctx, "outbound connection to ", destination)
		return h.client.DialContext(ctx, network, destination)
	case N.NetworkUDP:
		h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
		packetConn, err := h.client.ListenPacket(ctx, destination)
		if err != nil {
			return nil, err
		}
		return bufio.NewBindPacketConn(packetConn, destination), nil
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
}

func (h *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination
	h.logger.InfoContext(ctx, "outbound packet connection to ", destination)
	return h.client.ListenPacket(ctx, destination)
}
