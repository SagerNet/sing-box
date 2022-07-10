//go:build !no_tun

package inbound

import (
	"context"
	"net"
	"net/netip"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tun"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
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
	logger         log.Logger
	inboundOptions option.InboundOptions
	tunName        string
	tunMTU         uint32
	inet4Address   netip.Prefix
	inet6Address   netip.Prefix
	autoRoute      bool
	hijackDNS      bool

	tunFd uintptr
	tun   *tun.GVisorTun
}

func NewTun(ctx context.Context, router adapter.Router, logger log.Logger, tag string, options option.TunInboundOptions) (*Tun, error) {
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
		inet4Address:   netip.Prefix(options.Inet4Address),
		inet6Address:   netip.Prefix(options.Inet6Address),
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
	tunFd, err := tun.Open(t.tunName)
	if err != nil {
		return E.Cause(err, "create tun interface")
	}
	err = tun.Configure(t.tunName, t.inet4Address, t.inet6Address, t.tunMTU, t.autoRoute)
	if err != nil {
		return E.Cause(err, "configure tun interface")
	}
	t.tunFd = tunFd
	t.tun = tun.NewGVisor(t.ctx, tunFd, t.tunMTU, t)
	err = t.tun.Start()
	if err != nil {
		return err
	}
	t.logger.Info("started at ", t.tunName)
	return nil
}

func (t *Tun) Close() error {
	err := tun.UnConfigure(t.tunName, t.inet4Address, t.inet6Address, t.autoRoute)
	if err != nil {
		return err
	}
	return E.Errors(
		t.tun.Close(),
		os.NewFile(t.tunFd, "tun").Close(),
	)
}

func (t *Tun) NewConnection(ctx context.Context, conn net.Conn, upstreamMetadata M.Metadata) error {
	var metadata adapter.InboundContext
	metadata.Inbound = t.tag
	metadata.Network = C.NetworkTCP
	metadata.Source = upstreamMetadata.Source
	metadata.Destination = upstreamMetadata.Destination
	metadata.SniffEnabled = t.inboundOptions.SniffEnabled
	metadata.SniffOverrideDestination = t.inboundOptions.SniffOverrideDestination
	metadata.DomainStrategy = C.DomainStrategy(t.inboundOptions.DomainStrategy)
	if t.hijackDNS && upstreamMetadata.Destination.Port == 53 {
		return task.Run(ctx, func() error {
			return NewDNSConnection(ctx, t.router, t.logger, conn, metadata)
		})
	}
	t.logger.WithContext(ctx).Info("inbound connection from ", metadata.Source)
	t.logger.WithContext(ctx).Info("inbound connection to ", metadata.Destination)
	return t.router.RouteConnection(ctx, conn, metadata)
}

func (t *Tun) NewPacketConnection(ctx context.Context, conn N.PacketConn, upstreamMetadata M.Metadata) error {
	var metadata adapter.InboundContext
	metadata.Inbound = t.tag
	metadata.Network = C.NetworkUDP
	metadata.Source = upstreamMetadata.Source
	metadata.Destination = upstreamMetadata.Destination
	metadata.SniffEnabled = t.inboundOptions.SniffEnabled
	metadata.SniffOverrideDestination = t.inboundOptions.SniffOverrideDestination
	metadata.DomainStrategy = C.DomainStrategy(t.inboundOptions.DomainStrategy)
	if t.hijackDNS && upstreamMetadata.Destination.Port == 53 {
		return task.Run(ctx, func() error {
			return NewDNSPacketConnection(ctx, t.router, t.logger, conn, metadata)
		})
	}
	t.logger.WithContext(ctx).Info("inbound packet connection from ", metadata.Source)
	t.logger.WithContext(ctx).Info("inbound packet connection to ", metadata.Destination)
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
