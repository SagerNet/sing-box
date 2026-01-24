package option

import (
	"net/netip"

	"github.com/sagernet/sing/common/json/badoption"
)

type WireGuardEndpointOptions struct {
	System     bool                             `json:"system,omitempty"`
	Name       string                           `json:"name,omitempty"`
	MTU        uint32                           `json:"mtu,omitempty"`
	Address    badoption.Listable[netip.Prefix] `json:"address"`
	PrivateKey string                           `json:"private_key"`
	ListenPort uint16                           `json:"listen_port,omitempty"`
	Peers      []WireGuardPeer                  `json:"peers,omitempty"`
	UDPTimeout badoption.Duration               `json:"udp_timeout,omitempty"`
	Workers    int                              `json:"workers,omitempty"`
	DialerOptions
}

type WireGuardPeer struct {
	Address                     string                           `json:"address,omitempty"`
	Port                        uint16                           `json:"port,omitempty"`
	PublicKey                   string                           `json:"public_key,omitempty"`
	PreSharedKey                string                           `json:"pre_shared_key,omitempty"`
	AllowedIPs                  badoption.Listable[netip.Prefix] `json:"allowed_ips,omitempty"`
	PersistentKeepaliveInterval uint16                           `json:"persistent_keepalive_interval,omitempty"`
	Reserved                    []uint8                          `json:"reserved,omitempty"`
}

type LegacyWireGuardOutboundOptions struct {
	DialerOptions
	SystemInterface bool                             `json:"system_interface,omitempty"`
	GSO             bool                             `json:"gso,omitempty"`
	InterfaceName   string                           `json:"interface_name,omitempty"`
	LocalAddress    badoption.Listable[netip.Prefix] `json:"local_address"`
	PrivateKey      string                           `json:"private_key"`
	Peers           []LegacyWireGuardPeer            `json:"peers,omitempty"`
	ServerOptions
	PeerPublicKey string      `json:"peer_public_key"`
	PreSharedKey  string      `json:"pre_shared_key,omitempty"`
	Reserved      []uint8     `json:"reserved,omitempty"`
	Workers       int         `json:"workers,omitempty"`
	MTU           uint32      `json:"mtu,omitempty"`
	Network       NetworkList `json:"network,omitempty"`
}

type LegacyWireGuardPeer struct {
	ServerOptions
	PublicKey    string                           `json:"public_key,omitempty"`
	PreSharedKey string                           `json:"pre_shared_key,omitempty"`
	AllowedIPs   badoption.Listable[netip.Prefix] `json:"allowed_ips,omitempty"`
	Reserved     []uint8                          `json:"reserved,omitempty"`
}
