//go:build with_quic

package inbound

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/hysteria2"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Inbound = (*Hysteria2)(nil)

type Hysteria2 struct {
	myInboundAdapter
	tlsConfig tls.ServerConfig
	server    *hysteria2.Server
}

func NewHysteria2(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.Hysteria2InboundOptions) (*Hysteria2, error) {
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, C.ErrTLSRequired
	}
	tlsConfig, err := tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}
	var salamanderPassword string
	if options.Obfs != nil {
		if options.Obfs.Password == "" {
			return nil, E.New("missing obfs password")
		}
		switch options.Obfs.Type {
		case hysteria2.ObfsTypeSalamander:
			salamanderPassword = options.Obfs.Password
		default:
			return nil, E.New("unknown obfs type: ", options.Obfs.Type)
		}
	}
	var masqueradeHandler http.Handler
	if options.Masquerade != "" {
		masqueradeURL, err := url.Parse(options.Masquerade)
		if err != nil {
			return nil, E.Cause(err, "parse masquerade URL")
		}
		switch masqueradeURL.Scheme {
		case "file":
			masqueradeHandler = http.FileServer(http.Dir(masqueradeURL.Path))
		case "http", "https":
			masqueradeHandler = &httputil.ReverseProxy{
				Rewrite: func(r *httputil.ProxyRequest) {
					r.SetURL(masqueradeURL)
					r.Out.Host = r.In.Host
				},
				ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
					w.WriteHeader(http.StatusBadGateway)
				},
			}
		default:
			return nil, E.New("unknown masquerade URL scheme: ", masqueradeURL.Scheme)
		}
	}
	inbound := &Hysteria2{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeHysteria2,
			network:       []string{N.NetworkUDP},
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
		tlsConfig: tlsConfig,
	}
	server, err := hysteria2.NewServer(hysteria2.ServerOptions{
		Context:            ctx,
		Logger:             logger,
		SendBPS:            uint64(options.UpMbps * 1024 * 1024),
		ReceiveBPS:         uint64(options.DownMbps * 1024 * 1024),
		SalamanderPassword: salamanderPassword,
		TLSConfig:          tlsConfig,
		Users: common.Map(options.Users, func(it option.Hysteria2User) hysteria2.User {
			return hysteria2.User(it)
		}),
		IgnoreClientBandwidth: options.IgnoreClientBandwidth,
		Handler:               adapter.NewUpstreamHandler(adapter.InboundContext{}, inbound.newConnection, inbound.newPacketConnection, nil),
		MasqueradeHandler:     masqueradeHandler,
	})
	if err != nil {
		return nil, err
	}
	inbound.server = server
	return inbound, nil
}

func (h *Hysteria2) newConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	ctx = log.ContextWithNewID(ctx)
	h.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	metadata = h.createMetadata(conn, metadata)
	metadata.User, _ = auth.UserFromContext[string](ctx)
	return h.router.RouteConnection(ctx, conn, metadata)
}

func (h *Hysteria2) newPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	ctx = log.ContextWithNewID(ctx)
	metadata = h.createPacketMetadata(conn, metadata)
	metadata.User, _ = auth.UserFromContext[string](ctx)
	h.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	return h.router.RoutePacketConnection(ctx, conn, metadata)
}

func (h *Hysteria2) Start() error {
	if h.tlsConfig != nil {
		err := h.tlsConfig.Start()
		if err != nil {
			return err
		}
	}
	packetConn, err := h.myInboundAdapter.ListenUDP()
	if err != nil {
		return err
	}
	return h.server.Start(packetConn)
}

func (h *Hysteria2) Close() error {
	return common.Close(
		&h.myInboundAdapter,
		h.tlsConfig,
		common.PtrOrNil(h.server),
	)
}
