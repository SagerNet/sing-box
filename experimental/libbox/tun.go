package libbox

import (
	"net"
	"net/netip"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

type TunOptions interface {
	GetInet4Address() RoutePrefixIterator
	GetInet6Address() RoutePrefixIterator
	GetDNSServerAddress() (string, error)
	GetMTU() int32
	GetAutoRoute() bool
	GetStrictRoute() bool
	GetInet4RouteAddress() RoutePrefixIterator
	GetInet6RouteAddress() RoutePrefixIterator
	GetIncludePackage() StringIterator
	GetExcludePackage() StringIterator
	IsHTTPProxyEnabled() bool
	GetHTTPProxyServer() string
	GetHTTPProxyServerPort() int32
}

type RoutePrefix struct {
	Address string
	Prefix  int32
}

func (p *RoutePrefix) Mask() string {
	var bits int
	if M.ParseSocksaddr(p.Address).Addr.Is6() {
		bits = 128
	} else {
		bits = 32
	}
	return net.IP(net.CIDRMask(int(p.Prefix), bits)).String()
}

type RoutePrefixIterator interface {
	Next() *RoutePrefix
	HasNext() bool
}

func mapRoutePrefix(prefixes []netip.Prefix) RoutePrefixIterator {
	return newIterator(common.Map(prefixes, func(prefix netip.Prefix) *RoutePrefix {
		return &RoutePrefix{
			Address: prefix.Addr().String(),
			Prefix:  int32(prefix.Bits()),
		}
	}))
}

var _ TunOptions = (*tunOptions)(nil)

type tunOptions struct {
	*tun.Options
	option.TunPlatformOptions
}

func (o *tunOptions) GetInet4Address() RoutePrefixIterator {
	return mapRoutePrefix(o.Inet4Address)
}

func (o *tunOptions) GetInet6Address() RoutePrefixIterator {
	return mapRoutePrefix(o.Inet6Address)
}

func (o *tunOptions) GetDNSServerAddress() (string, error) {
	if len(o.Inet4Address) == 0 || o.Inet4Address[0].Bits() == 32 {
		return "", E.New("need one more IPv4 address for DNS hijacking")
	}
	return o.Inet4Address[0].Addr().Next().String(), nil
}

func (o *tunOptions) GetMTU() int32 {
	return int32(o.MTU)
}

func (o *tunOptions) GetAutoRoute() bool {
	return o.AutoRoute
}

func (o *tunOptions) GetStrictRoute() bool {
	return o.StrictRoute
}

func (o *tunOptions) GetInet4RouteAddress() RoutePrefixIterator {
	return mapRoutePrefix(o.Inet4RouteAddress)
}

func (o *tunOptions) GetInet6RouteAddress() RoutePrefixIterator {
	return mapRoutePrefix(o.Inet6RouteAddress)
}

func (o *tunOptions) GetIncludePackage() StringIterator {
	return newIterator(o.IncludePackage)
}

func (o *tunOptions) GetExcludePackage() StringIterator {
	return newIterator(o.ExcludePackage)
}

func (o *tunOptions) IsHTTPProxyEnabled() bool {
	if o.TunPlatformOptions.HTTPProxy == nil {
		return false
	}
	return o.TunPlatformOptions.HTTPProxy.Enabled
}

func (o *tunOptions) GetHTTPProxyServer() string {
	return o.TunPlatformOptions.HTTPProxy.Server
}

func (o *tunOptions) GetHTTPProxyServerPort() int32 {
	return int32(o.TunPlatformOptions.HTTPProxy.ServerPort)
}
