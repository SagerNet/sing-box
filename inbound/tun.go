package inbound

import (
	"context"
	"net"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ranges"
)

var _ adapter.Inbound = (*Tun)(nil)

type Tun struct {
	tag                    string
	ctx                    context.Context
	router                 adapter.Router
	logger                 log.ContextLogger
	inboundOptions         option.InboundOptions
	tunOptions             tun.Options
	endpointIndependentNat bool
	udpTimeout             int64
	stack                  string
	tunIf                  tun.Tun
	tunStack               tun.Stack
	platformInterface      platform.Interface
	platformOptions        option.TunPlatformOptions
}

func NewTun(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TunInboundOptions, platformInterface platform.Interface) (*Tun, error) {
	tunMTU := options.MTU
	if tunMTU == 0 {
		tunMTU = 9000
	}
	gsoMaxSize := options.GSOMaxSize
	if gsoMaxSize == 0 {
		gsoMaxSize = 65536
	}
	var udpTimeout int64
	if options.UDPTimeout != 0 {
		udpTimeout = options.UDPTimeout
	} else {
		udpTimeout = int64(C.UDPTimeout.Seconds())
	}
	includeUID := uidToRange(options.IncludeUID)
	if len(options.IncludeUIDRange) > 0 {
		var err error
		includeUID, err = parseRange(includeUID, options.IncludeUIDRange)
		if err != nil {
			return nil, E.Cause(err, "parse include_uid_range")
		}
	}
	excludeUID := uidToRange(options.ExcludeUID)
	if len(options.ExcludeUIDRange) > 0 {
		var err error
		excludeUID, err = parseRange(excludeUID, options.ExcludeUIDRange)
		if err != nil {
			return nil, E.Cause(err, "parse exclude_uid_range")
		}
	}
	return &Tun{
		tag:            tag,
		ctx:            ctx,
		router:         router,
		logger:         logger,
		inboundOptions: options.InboundOptions,
		tunOptions: tun.Options{
			Name:                     options.InterfaceName,
			MTU:                      tunMTU,
			GSO:                      options.GSO,
			GSOMaxSize:               gsoMaxSize,
			Inet4Address:             options.Inet4Address,
			Inet6Address:             options.Inet6Address,
			AutoRoute:                options.AutoRoute,
			StrictRoute:              options.StrictRoute,
			IncludeInterface:         options.IncludeInterface,
			ExcludeInterface:         options.ExcludeInterface,
			Inet4RouteAddress:        options.Inet4RouteAddress,
			Inet6RouteAddress:        options.Inet6RouteAddress,
			Inet4RouteExcludeAddress: options.Inet4RouteExcludeAddress,
			Inet6RouteExcludeAddress: options.Inet6RouteExcludeAddress,
			IncludeUID:               includeUID,
			ExcludeUID:               excludeUID,
			IncludeAndroidUser:       options.IncludeAndroidUser,
			IncludePackage:           options.IncludePackage,
			ExcludePackage:           options.ExcludePackage,
			InterfaceMonitor:         router.InterfaceMonitor(),
			TableIndex:               2022,
		},
		endpointIndependentNat: options.EndpointIndependentNat,
		udpTimeout:             udpTimeout,
		stack:                  options.Stack,
		platformInterface:      platformInterface,
		platformOptions:        common.PtrValueOrDefault(options.Platform),
	}, nil
}

func uidToRange(uidList option.Listable[uint32]) []ranges.Range[uint32] {
	return common.Map(uidList, func(uid uint32) ranges.Range[uint32] {
		return ranges.NewSingle(uid)
	})
}

func parseRange(uidRanges []ranges.Range[uint32], rangeList []string) ([]ranges.Range[uint32], error) {
	for _, uidRange := range rangeList {
		if !strings.Contains(uidRange, ":") {
			return nil, E.New("missing ':' in range: ", uidRange)
		}
		subIndex := strings.Index(uidRange, ":")
		if subIndex == 0 {
			return nil, E.New("missing range start: ", uidRange)
		} else if subIndex == len(uidRange)-1 {
			return nil, E.New("missing range end: ", uidRange)
		}
		var start, end uint64
		var err error
		start, err = strconv.ParseUint(uidRange[:subIndex], 10, 32)
		if err != nil {
			return nil, E.Cause(err, "parse range start")
		}
		end, err = strconv.ParseUint(uidRange[subIndex+1:], 10, 32)
		if err != nil {
			return nil, E.Cause(err, "parse range end")
		}
		uidRanges = append(uidRanges, ranges.New(uint32(start), uint32(end)))
	}
	return uidRanges, nil
}

func (t *Tun) Type() string {
	return C.TypeTun
}

func (t *Tun) Tag() string {
	return t.tag
}

func (t *Tun) Start() error {
	if C.IsAndroid && t.platformInterface == nil {
		t.tunOptions.BuildAndroidRules(t.router.PackageManager(), t)
	}
	if t.tunOptions.Name == "" {
		t.tunOptions.Name = tun.CalculateInterfaceName("")
	}
	var (
		tunInterface tun.Tun
		err          error
	)
	monitor := taskmonitor.New(t.logger, C.DefaultStartTimeout)
	monitor.Start("open tun interface")
	if t.platformInterface != nil {
		tunInterface, err = t.platformInterface.OpenTun(&t.tunOptions, t.platformOptions)
	} else {
		tunInterface, err = tun.New(t.tunOptions)
	}
	monitor.Finish()
	if err != nil {
		return E.Cause(err, "configure tun interface")
	}
	t.logger.Trace("creating stack")
	t.tunIf = tunInterface
	t.tunStack, err = tun.NewStack(t.stack, tun.StackOptions{
		Context:                t.ctx,
		Tun:                    tunInterface,
		TunOptions:             t.tunOptions,
		EndpointIndependentNat: t.endpointIndependentNat,
		UDPTimeout:             t.udpTimeout,
		Handler:                t,
		Logger:                 t.logger,
		ForwarderBindInterface: t.platformInterface != nil,
		InterfaceFinder:        t.router.InterfaceFinder(),
	})
	if err != nil {
		return err
	}
	monitor.Start("initiating tun stack")
	err = t.tunStack.Start()
	monitor.Finish()
	if err != nil {
		return err
	}
	t.logger.Info("started at ", t.tunOptions.Name)
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
	metadata.Source = upstreamMetadata.Source
	metadata.Destination = upstreamMetadata.Destination
	metadata.InboundOptions = t.inboundOptions
	t.logger.InfoContext(ctx, "inbound connection from ", metadata.Source)
	t.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	err := t.router.RouteConnection(ctx, conn, metadata)
	if err != nil {
		t.NewError(ctx, err)
	}
	return nil
}

func (t *Tun) NewPacketConnection(ctx context.Context, conn N.PacketConn, upstreamMetadata M.Metadata) error {
	ctx = log.ContextWithNewID(ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = t.tag
	metadata.InboundType = C.TypeTun
	metadata.Source = upstreamMetadata.Source
	metadata.Destination = upstreamMetadata.Destination
	metadata.InboundOptions = t.inboundOptions
	t.logger.InfoContext(ctx, "inbound packet connection from ", metadata.Source)
	t.logger.InfoContext(ctx, "inbound packet connection to ", metadata.Destination)
	err := t.router.RoutePacketConnection(ctx, conn, metadata)
	if err != nil {
		t.NewError(ctx, err)
	}
	return nil
}

func (t *Tun) NewError(ctx context.Context, err error) {
	NewError(t.logger, ctx, err)
}
