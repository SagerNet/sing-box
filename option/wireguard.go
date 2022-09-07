package option

type WireGuardOutboundOptions struct {
	DialerOptions
	ServerOptions
	SystemInterface bool                   `json:"system_interface,omitempty"`
	InterfaceName   string                 `json:"interface_name,omitempty"`
	LocalAddress    Listable[ListenPrefix] `json:"local_address"`
	PrivateKey      string                 `json:"private_key"`
	PeerPublicKey   string                 `json:"peer_public_key"`
	PreSharedKey    string                 `json:"pre_shared_key,omitempty"`
	MTU             uint32                 `json:"mtu,omitempty"`
	Network         NetworkList            `json:"network,omitempty"`
}
