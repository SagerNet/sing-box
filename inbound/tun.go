//go:build linux || windows

package inbound

import (
	"context"
	"net"
	"net/netip"
	"runtime"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/task"
)

var _ adapter.Inbound = (*Tun)(nil)

type Tun struct {
	tag string

	ctx            context.Context
	router         adapter.Router
	logger         log.ContextLogger
	inboundOptions option.InboundOptions
	tunName        string
	tunMTU         uint32
	inet4Address   netip.Prefix
	inet6Address   netip.Prefix
	autoRoute      bool
	hijackDNS      bool

	tunIf    tun.Tun
	tunStack *tun.GVisorTun
}

func NewTun(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TunInboundOptions) (*Tun, error) {
	tunName := options.InterfaceName
	if tunName == "" {
		tunName = mkInterfaceName()
	}
	tunMTU := options.MTU
	if tunMTU == 0 {
		tunMTU = 1500
	}
	return &Tun{
		tag:            tag,
		ctx:            ctx,
		router:         router,
		logger:         logger,
		inboundOptions: options.InboundOptions,
		tunName:        tunName,
		tunMTU:         tunMTU,
		inet4Address:   options.Inet4Address.Build(),
		inet6Address:   options.Inet6Address.Build(),
		autoRoute:      options.AutoRoute,
		hijackDNS:      options.HijackDNS,
	}, nil
}

func (t *Tun) Type() string {
	return C.TypeTun
}

func (t *Tun) Tag() string {
	return t.tag
}

func (t *Tun) Start() error {
	tunIf, err := tun.Open(t.tunName, t.inet4Address, t.inet6Address, t.tunMTU, t.autoRoute)
	if err != nil {
		return E.Cause(err, "configure tun interface")
	}
	t.tunIf = tunIf
	t.tunStack = tun.NewGVisor(t.ctx, tunIf, t.tunMTU, t)
	err = t.tunStack.Start()
	if err != nil {
		return err
	}
	t.logger.Info("started at ", t.tunName)
	return nil
}

func (t *Tun) Close() error {
	return common.Close(
		t.tunStack,
		t.tunIf,
	)
}

func (t *Tun) NewConnection(ctx context.Context, conn net.Conn, upstreamMetadata M.Metadata) error {
	ctx = log.ContextWithNewID(ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = t.tag
	metadata.InboundType = C.TypeTun
	metadata.Network = C.NetworkTCP
	metadata.Source = upstreamMetadata.Source
	metadata.Destination = upstreamMetadata.Destination
	metadata.SniffEnabled = t.inboundOptions.SniffEnabled
	metadata.SniffOverrideDestination = t.inboundOptions.SniffOverrideDestination
	metadata.DomainStrategy = dns.DomainStrategy(t.inboundOptions.DomainStrategy)
	if t.hijackDNS && upstreamMetadata.Destination.Port == 53 {
		return task.Run(ctx, func() error {
			return NewDNSConnection(ctx, t.router, t.logger, conn, metadata)
		})
	}
	t.logger.InfoContext(ctx, "inbound connection from ", metadata.Source)
	t.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	return t.router.RouteConnection(ctx, conn, metadata)
}

func (t *Tun) NewPacketConnection(ctx context.Context, conn N.PacketConn, upstreamMetadata M.Metadata) error {
	ctx = log.ContextWithNewID(ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = t.tag
	metadata.InboundType = C.TypeTun
	metadata.Network = C.NetworkUDP
	metadata.Source = upstreamMetadata.Source
	metadata.Destination = upstreamMetadata.Destination
	metadata.SniffEnabled = t.inboundOptions.SniffEnabled
	metadata.SniffOverrideDestination = t.inboundOptions.SniffOverrideDestination
	metadata.DomainStrategy = dns.DomainStrategy(t.inboundOptions.DomainStrategy)
	if t.hijackDNS && upstreamMetadata.Destination.Port == 53 {
		return task.Run(ctx, func() error {
			return NewDNSPacketConnection(ctx, t.router, t.logger, conn, metadata)
		})
	}
	t.logger.InfoContext(ctx, "inbound packet connection from ", metadata.Source)
	t.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	return t.router.RoutePacketConnection(ctx, conn, metadata)
}

func (t *Tun) NewError(ctx context.Context, err error) {
	NewError(t.logger, ctx, err)
}

func mkInterfaceName() (tunName string) {
	switch runtime.GOOS {
	case "darwin":
		tunName = "utun"
	default:
		tunName = "tun"
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return
	}
	var tunIndex int
	for _, netInterface := range interfaces {
		if strings.HasPrefix(netInterface.Name, tunName) {
			index, parseErr := strconv.ParseInt(netInterface.Name[len(tunName):], 10, 16)
			if parseErr == nil {
				tunIndex = int(index) + 1
			}
		}
	}
	tunName = F.ToString(tunName, tunIndex)
	return
}
