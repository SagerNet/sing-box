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
)

var _ adapter.Inbound = (*Tun)(nil)

type Tun struct {
	tag string

	ctx     context.Context
	router  adapter.Router
	logger  log.Logger
	options option.TunInboundOptions

	tunName string
	tunFd   uintptr
	tun     *tun.GVisorTun
}

func NewTun(ctx context.Context, router adapter.Router, logger log.Logger, tag string, options option.TunInboundOptions) (*Tun, error) {
	return &Tun{
		tag:     tag,
		ctx:     ctx,
		router:  router,
		logger:  logger,
		options: options,
	}, nil
}

func (t *Tun) Type() string {
	return C.TypeTun
}

func (t *Tun) Tag() string {
	return t.tag
}

func (t *Tun) Start() error {
	tunName := t.options.InterfaceName
	if tunName == "" {
		tunName = mkInterfaceName()
	}
	var mtu uint32
	if t.options.MTU != 0 {
		mtu = t.options.MTU
	} else {
		mtu = 1500
	}

	tunFd, err := tun.Open(tunName)
	if err != nil {
		return E.Cause(err, "create tun interface")
	}
	err = tun.Configure(tunName, netip.Prefix(t.options.Inet4Address), netip.Prefix(t.options.Inet6Address), mtu, t.options.AutoRoute)
	if err != nil {
		return E.Cause(err, "configure tun interface")
	}
	t.tunName = tunName
	t.tunFd = tunFd
	t.tun = tun.NewGVisor(t.ctx, tunFd, mtu, t)
	err = t.tun.Start()
	if err != nil {
		return err
	}
	t.logger.Info("started at ", tunName)
	return nil
}

func (t *Tun) Close() error {
	err := tun.UnConfigure(t.tunName, netip.Prefix(t.options.Inet4Address), netip.Prefix(t.options.Inet6Address), t.options.AutoRoute)
	if err != nil {
		return err
	}
	return E.Errors(
		t.tun.Close(),
		os.NewFile(t.tunFd, "tun").Close(),
	)
}

func (t *Tun) NewConnection(ctx context.Context, conn net.Conn, upstreamMetadata M.Metadata) error {
	t.logger.WithContext(ctx).Info("inbound connection from ", upstreamMetadata.Source)
	t.logger.WithContext(ctx).Info("inbound connection to ", upstreamMetadata.Destination)
	var metadata adapter.InboundContext
	metadata.Inbound = t.tag
	metadata.Network = C.NetworkTCP
	metadata.Source = upstreamMetadata.Source
	metadata.Destination = upstreamMetadata.Destination
	metadata.SniffEnabled = t.options.SniffEnabled
	metadata.SniffOverrideDestination = t.options.SniffOverrideDestination
	metadata.DomainStrategy = C.DomainStrategy(t.options.DomainStrategy)
	return t.router.RouteConnection(ctx, conn, metadata)
}

func (t *Tun) NewPacketConnection(ctx context.Context, conn N.PacketConn, upstreamMetadata M.Metadata) error {
	t.logger.WithContext(ctx).Info("inbound packet connection from ", upstreamMetadata.Source)
	t.logger.WithContext(ctx).Info("inbound packet connection to ", upstreamMetadata.Destination)
	var metadata adapter.InboundContext
	metadata.Inbound = t.tag
	metadata.Network = C.NetworkUDP
	metadata.Source = upstreamMetadata.Source
	metadata.Destination = upstreamMetadata.Destination
	metadata.SniffEnabled = t.options.SniffEnabled
	metadata.SniffOverrideDestination = t.options.SniffOverrideDestination
	metadata.DomainStrategy = C.DomainStrategy(t.options.DomainStrategy)
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
