package option

import "github.com/sagernet/sing/common/json/badoption"

type CloudflaredInboundOptions struct {
	Token                       string             `json:"token,omitempty"`
	HighAvailabilityConnections int                `json:"ha_connections,omitempty"`
	Protocol                    string             `json:"protocol,omitempty"`
	PostQuantum                 bool               `json:"post_quantum,omitempty"`
	EdgeIPVersion               int                `json:"edge_ip_version,omitempty"`
	DatagramVersion             string             `json:"datagram_version,omitempty"`
	GracePeriod                 badoption.Duration `json:"grace_period,omitempty"`
	Region                      string             `json:"region,omitempty"`
	ControlDialer               DialerOptions      `json:"control_dialer,omitempty"`
	TunnelDialer                DialerOptions      `json:"tunnel_dialer,omitempty"`
}
