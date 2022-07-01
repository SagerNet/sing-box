package config

type Route struct {
	Type string `json:"type"`
}

type SimpleRule struct {
	Inbound   []string `json:"inbound,omitempty"`
	IPVersion []int    `json:"ip_version,omitempty"`
	Network   []string `json:"network,omitempty"`
	Protocol  []string `json:"protocol,omitempty"`
	Domain    []string `json:"domain,omitempty"`
	Outbound  string   `json:"outbound,omitempty"`
}
