package option

import (
	"net/netip"

	"github.com/sagernet/sing/common/json/badoption"
)

/*
WireGuardAdvancedSecurityOptions provides advanced security options for WireGuard required to activate AmneziaWG.

In AmneziaWG, random bytes are appended to every auth packet to alter their size.
Thus, "init and response handshake packets" have added "junk" at the beginning of their data, the size of which
is determined by the values S1 and S2.
By default, the initiating handshake packet has a fixed size (148 bytes). After adding the junk, its size becomes 148 bytes + S1.
AmneziaWG also incorporates another trick for more reliable masking. Before initiating a session, Amnezia sends a
certain number of "junk" packets to thoroughly confuse DPI systems. The number of these packets and their
minimum and maximum byte sizes can also be adjusted in the settings, using parameters Jc, Jmin, and Jmax.
*/
type WireGuardAdvancedSecurityOptions struct {
	JunkPacketCount            int    `json:"junk_packet_count,omitempty"`             // jc
	JunkPacketMinSize          int    `json:"junk_packet_min_size,omitempty"`          // jmin
	JunkPacketMaxSize          int    `json:"junk_packet_max_size,omitempty"`          // jmax
	InitPacketJunkSize         int    `json:"init_packet_junk_size,omitempty"`         // s1
	ResponsePacketJunkSize     int    `json:"response_packet_junk_size,omitempty"`     // s2
	InitPacketMagicHeader      uint32 `json:"init_packet_magic_header,omitempty"`      // h1
	ResponsePacketMagicHeader  uint32 `json:"response_packet_magic_header,omitempty"`  // h2
	UnderloadPacketMagicHeader uint32 `json:"underload_packet_magic_header,omitempty"` // h3
	TransportPacketMagicHeader uint32 `json:"transport_packet_magic_header,omitempty"` // h4
}

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
	WireGuardAdvancedSecurityOptions
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
