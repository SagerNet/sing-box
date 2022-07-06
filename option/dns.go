package option

type DNSOptions struct {
	Servers []DNSServerOptions `json:"servers,omitempty"`
}

type DNSServerOptions struct {
	Tag             string `json:"tag,omitempty"`
	Address         string `json:"address"`
	Detour          string `json:"detour,omitempty"`
	AddressResolver string `json:"address_resolver,omitempty"`
}
