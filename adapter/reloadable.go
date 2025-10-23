package adapter

// ReloadableInbound is an inbound that supports hot reloading its configuration
// without disrupting existing connections.
type ReloadableInbound interface {
	Inbound
	// Reload updates the inbound configuration without closing existing connections.
	// The options parameter should be the same type used in the inbound's constructor.
	Reload(options any) error
}

// ReloadableOutbound is an outbound that supports hot reloading its configuration
// without disrupting existing connections.
type ReloadableOutbound interface {
	Outbound
	// Reload updates the outbound configuration without closing existing connections.
	// The options parameter should be the same type used in the outbound's constructor.
	Reload(options any) error
}

// ReloadableEndpoint is an endpoint that supports hot reloading its configuration
// without disrupting existing connections.
type ReloadableEndpoint interface {
	Endpoint
	// Reload updates the endpoint configuration without closing existing connections.
	// The options parameter should be the same type used in the endpoint's constructor.
	Reload(options any) error
}
