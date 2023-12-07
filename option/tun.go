package option

import "net/netip"

type TunInboundOptions struct {
	InterfaceName            string                 `json:"interface_name,omitempty"`
	MTU                      uint32                 `json:"mtu,omitempty"`
	GSO                      bool                   `json:"gso,omitempty"`
	GSOMaxSize               uint32                 `json:"gso_max_size,omitempty"`
	Inet4Address             Listable[netip.Prefix] `json:"inet4_address,omitempty"`
	Inet6Address             Listable[netip.Prefix] `json:"inet6_address,omitempty"`
	AutoRoute                bool                   `json:"auto_route,omitempty"`
	StrictRoute              bool                   `json:"strict_route,omitempty"`
	Inet4RouteAddress        Listable[netip.Prefix] `json:"inet4_route_address,omitempty"`
	Inet6RouteAddress        Listable[netip.Prefix] `json:"inet6_route_address,omitempty"`
	Inet4RouteExcludeAddress Listable[netip.Prefix] `json:"inet4_route_exclude_address,omitempty"`
	Inet6RouteExcludeAddress Listable[netip.Prefix] `json:"inet6_route_exclude_address,omitempty"`
	IncludeInterface         Listable[string]       `json:"include_interface,omitempty"`
	ExcludeInterface         Listable[string]       `json:"exclude_interface,omitempty"`
	IncludeUID               Listable[uint32]       `json:"include_uid,omitempty"`
	IncludeUIDRange          Listable[string]       `json:"include_uid_range,omitempty"`
	ExcludeUID               Listable[uint32]       `json:"exclude_uid,omitempty"`
	ExcludeUIDRange          Listable[string]       `json:"exclude_uid_range,omitempty"`
	IncludeAndroidUser       Listable[int]          `json:"include_android_user,omitempty"`
	IncludePackage           Listable[string]       `json:"include_package,omitempty"`
	ExcludePackage           Listable[string]       `json:"exclude_package,omitempty"`
	EndpointIndependentNat   bool                   `json:"endpoint_independent_nat,omitempty"`
	UDPTimeout               int64                  `json:"udp_timeout,omitempty"`
	Stack                    string                 `json:"stack,omitempty"`
	Platform                 *TunPlatformOptions    `json:"platform,omitempty"`
	InboundOptions
}
