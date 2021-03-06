package option

type TunInboundOptions struct {
	InterfaceName          string        `json:"interface_name,omitempty"`
	MTU                    uint32        `json:"mtu,omitempty"`
	Inet4Address           *ListenPrefix `json:"inet4_address,omitempty"`
	Inet6Address           *ListenPrefix `json:"inet6_address,omitempty"`
	AutoRoute              bool          `json:"auto_route,omitempty"`
	EndpointIndependentNat bool          `json:"endpoint_independent_nat,omitempty"`
	UDPTimeout             int64         `json:"udp_timeout,omitempty"`
	InboundOptions
}
