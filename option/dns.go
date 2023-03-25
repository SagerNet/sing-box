package option

type DNSOptions struct {
	Servers        []DNSServerOptions `json:"servers,omitempty"`
	Rules          []DNSRule          `json:"rules,omitempty"`
	Final          string             `json:"final,omitempty"`
	ReverseMapping bool               `json:"reverse_mapping,omitempty"`
	FakeIP         *DNSFakeIPOptions  `json:"fakeip,omitempty"`
	DNSClientOptions
}

type DNSServerOptions struct {
	Tag                  string         `json:"tag,omitempty"`
	Address              string         `json:"address"`
	AddressResolver      string         `json:"address_resolver,omitempty"`
	AddressStrategy      DomainStrategy `json:"address_strategy,omitempty"`
	AddressFallbackDelay Duration       `json:"address_fallback_delay,omitempty"`
	Strategy             DomainStrategy `json:"strategy,omitempty"`
	Detour               string         `json:"detour,omitempty"`
}

type DNSClientOptions struct {
	Strategy      DomainStrategy `json:"strategy,omitempty"`
	DisableCache  bool           `json:"disable_cache,omitempty"`
	DisableExpire bool           `json:"disable_expire,omitempty"`
}

type DNSFakeIPOptions struct {
	Enabled    bool          `json:"enabled,omitempty"`
	Inet4Range *ListenPrefix `json:"inet4_range,omitempty"`
	Inet6Range *ListenPrefix `json:"inet6_range,omitempty"`
}
