package option

import "net/netip"

type TunInboundOptions struct {
	InterfaceName          string                 `json:"interface_name,omitempty"`
	MTU                    uint32                 `json:"mtu,omitempty"`
	GSO                    bool                   `json:"gso,omitempty"`
	Address                Listable[netip.Prefix] `json:"address,omitempty"`
	AutoRoute              bool                   `json:"auto_route,omitempty"`
	AutoRedirect           bool                   `json:"auto_redirect,omitempty"`
	StrictRoute            bool                   `json:"strict_route,omitempty"`
	RouteAddress           Listable[netip.Prefix] `json:"route_address,omitempty"`
	RouteAddressSet        Listable[string]       `json:"route_address_set,omitempty"`
	RouteExcludeAddress    Listable[netip.Prefix] `json:"route_exclude_address,omitempty"`
	RouteExcludeAddressSet Listable[string]       `json:"route_exclude_address_set,omitempty"`
	IncludeInterface       Listable[string]       `json:"include_interface,omitempty"`
	ExcludeInterface       Listable[string]       `json:"exclude_interface,omitempty"`
	IncludeUID             Listable[uint32]       `json:"include_uid,omitempty"`
	IncludeUIDRange        Listable[string]       `json:"include_uid_range,omitempty"`
	ExcludeUID             Listable[uint32]       `json:"exclude_uid,omitempty"`
	ExcludeUIDRange        Listable[string]       `json:"exclude_uid_range,omitempty"`
	IncludeAndroidUser     Listable[int]          `json:"include_android_user,omitempty"`
	IncludePackage         Listable[string]       `json:"include_package,omitempty"`
	ExcludePackage         Listable[string]       `json:"exclude_package,omitempty"`
	EndpointIndependentNat bool                   `json:"endpoint_independent_nat,omitempty"`
	UDPTimeout             UDPTimeoutCompat       `json:"udp_timeout,omitempty"`
	Stack                  string                 `json:"stack,omitempty"`
	Platform               *TunPlatformOptions    `json:"platform,omitempty"`
	InboundOptions

	// Deprecated: merged to `address`
	Inet4Address Listable[netip.Prefix] `json:"inet4_address,omitempty"`
	// Deprecated: merged to `address`
	Inet6Address Listable[netip.Prefix] `json:"inet6_address,omitempty"`
	// Deprecated: merged to `route_address`
	Inet4RouteAddress Listable[netip.Prefix] `json:"inet4_route_address,omitempty"`
	// Deprecated: merged to `route_address`
	Inet6RouteAddress Listable[netip.Prefix] `json:"inet6_route_address,omitempty"`
	// Deprecated: merged to `route_exclude_address`
	Inet4RouteExcludeAddress Listable[netip.Prefix] `json:"inet4_route_exclude_address,omitempty"`
	// Deprecated: merged to `route_exclude_address`
	Inet6RouteExcludeAddress Listable[netip.Prefix] `json:"inet6_route_exclude_address,omitempty"`
}
