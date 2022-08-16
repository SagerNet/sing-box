package option

type TunInboundOptions struct {
	InterfaceName          string           `json:"interface_name,omitempty"`
	MTU                    uint32           `json:"mtu,omitempty"`
	Inet4Address           *ListenPrefix    `json:"inet4_address,omitempty"`
	Inet6Address           *ListenPrefix    `json:"inet6_address,omitempty"`
	AutoRoute              bool             `json:"auto_route,omitempty"`
	IncludeUID             Listable[uint32] `json:"include_uid,omitempty"`
	IncludeUIDRange        Listable[string] `json:"include_uid_range,omitempty"`
	ExcludeUID             Listable[uint32] `json:"exclude_uid,omitempty"`
	ExcludeUIDRange        Listable[string] `json:"exclude_uid_range,omitempty"`
	IncludeAndroidUser     Listable[int]    `json:"include_android_user,omitempty"`
	IncludePackage         Listable[string] `json:"include_package,omitempty"`
	ExcludePackage         Listable[string] `json:"exclude_package,omitempty"`
	EndpointIndependentNat bool             `json:"endpoint_independent_nat,omitempty"`
	UDPTimeout             int64            `json:"udp_timeout,omitempty"`
	Stack                  string           `json:"stack,omitempty"`
	InboundOptions
}
