package option

import (
	"net/netip"

	"github.com/sagernet/sing/common/json/badoption"
)

type DNSOptions struct {
	Servers        []DNSServerOptions `json:"servers,omitempty"`
	Rules          []DNSRule          `json:"rules,omitempty"`
	Final          string             `json:"final,omitempty"`
	ReverseMapping bool               `json:"reverse_mapping,omitempty"`
	FakeIP         *DNSFakeIPOptions  `json:"fakeip,omitempty"`
	DNSClientOptions
}

type DNSServerOptions struct {
	Tag                  string                `json:"tag,omitempty"`
	Address              string                `json:"address"`
	AddressResolver      string                `json:"address_resolver,omitempty"`
	AddressStrategy      DomainStrategy        `json:"address_strategy,omitempty"`
	AddressFallbackDelay badoption.Duration    `json:"address_fallback_delay,omitempty"`
	Strategy             DomainStrategy        `json:"strategy,omitempty"`
	Detour               string                `json:"detour,omitempty"`
	ClientSubnet         *badoption.Prefixable `json:"client_subnet,omitempty"`
}

type DNSClientOptions struct {
	Strategy         DomainStrategy        `json:"strategy,omitempty"`
	DisableCache     bool                  `json:"disable_cache,omitempty"`
	DisableExpire    bool                  `json:"disable_expire,omitempty"`
	IndependentCache bool                  `json:"independent_cache,omitempty"`
	CacheCapacity    uint32                `json:"cache_capacity,omitempty"`
	ClientSubnet     *badoption.Prefixable `json:"client_subnet,omitempty"`
}

type DNSFakeIPOptions struct {
	Enabled    bool          `json:"enabled,omitempty"`
	Inet4Range *netip.Prefix `json:"inet4_range,omitempty"`
	Inet6Range *netip.Prefix `json:"inet6_range,omitempty"`
}
