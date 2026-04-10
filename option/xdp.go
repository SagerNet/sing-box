package option

import (
	"net/netip"

	"github.com/sagernet/sing/common/json/badoption"
)

type XDPInboundOptions struct {
	// Interface is the network interface to attach XDP program to.
	Interface string `json:"interface"`

	// Address is the local address for the gVisor netstack.
	// If empty, auto-detected from the network interface.
	Address badoption.Listable[netip.Prefix] `json:"address,omitempty"`

	// FrameSize is the UMEM frame size in bytes.
	// Default: 4096
	FrameSize uint32 `json:"frame_size,omitempty"`

	// FrameCount is the total number of UMEM frames (shared across all queues).
	// Default: 4096
	FrameCount uint32 `json:"frame_count,omitempty"`

	// MTU for the virtual network interface.
	// Default: 1500
	MTU uint32 `json:"mtu,omitempty"`

	// RouteAddress specifies destination IP prefixes to capture.
	// Only traffic to these CIDRs will be redirected to AF_XDP.
	// If empty, all traffic is captured (equivalent to 0.0.0.0/0 + ::/0).
	RouteAddress badoption.Listable[netip.Prefix] `json:"route_address,omitempty"`

	// RouteExcludeAddress specifies destination IP prefixes excluded from capture.
	// Traffic to these CIDRs will be passed to the kernel stack.
	RouteExcludeAddress badoption.Listable[netip.Prefix] `json:"route_exclude_address,omitempty"`

	UDPTimeout UDPTimeoutCompat `json:"udp_timeout,omitempty"`
}
