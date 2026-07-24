package option

import "github.com/sagernet/sing/common/json/badoption"

type CloudflaredInboundOptions struct {
	Token                       string             `json:"token,omitempty"`
	HighAvailabilityConnections int                `json:"ha_connections,omitempty"`
	Protocol                    string             `json:"protocol,omitempty" enum:"auto,quic,http2,h2mux"`
	PostQuantum                 bool               `json:"post_quantum,omitempty"`
	EdgeIPVersion               int                `json:"edge_ip_version,omitempty" enum:"0,4,6"`
	DatagramVersion             string             `json:"datagram_version,omitempty" enum:"v2,v3"`
	GracePeriod                 badoption.Duration `json:"grace_period,omitempty"`
	Region                      string             `json:"region,omitempty"`
	ControlDialer               DialerOptions      `json:"control_dialer,omitempty"`
	TunnelDialer                DialerOptions      `json:"tunnel_dialer,omitempty"`
}
