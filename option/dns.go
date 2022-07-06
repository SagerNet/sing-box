package option

type DNSOptions struct {
	Servers []DNSServerOptions `json:"servers,omitempty"`
	DNSClientOptions
}

type DNSClientOptions struct {
	DisableCache  bool `json:"disable_cache,omitempty"`
	DisableExpire bool `json:"disable_expire,omitempty"`
}

type DNSServerOptions struct {
	Tag             string `json:"tag,omitempty"`
	Address         string `json:"address"`
	AddressResolver string `json:"address_resolver,omitempty"`
	DialerOptions
}
