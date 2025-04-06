package option

import (
	"net/netip"
)

type TailscaleEndpointOptions struct {
	DialerOptions
	StateDirectory         string           `json:"state_directory,omitempty"`
	AuthKey                string           `json:"auth_key,omitempty"`
	ControlURL             string           `json:"control_url,omitempty"`
	Ephemeral              bool             `json:"ephemeral,omitempty"`
	Hostname               string           `json:"hostname,omitempty"`
	ExitNode               string           `json:"exit_node,omitempty"`
	ExitNodeAllowLANAccess bool             `json:"exit_node_allow_lan_access,omitempty"`
	AdvertiseRoutes        []netip.Prefix   `json:"advertise_routes,omitempty"`
	AdvertiseExitNode      bool             `json:"advertise_exit_node,omitempty"`
	UDPTimeout             UDPTimeoutCompat `json:"udp_timeout,omitempty"`
}

type TailscaleDNSServerOptions struct {
	Endpoint               string `json:"endpoint,omitempty"`
	AcceptDefaultResolvers bool   `json:"accept_default_resolvers,omitempty"`
}
