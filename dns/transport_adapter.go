package dns

import (
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

var _ adapter.LegacyDNSTransport = (*TransportAdapter)(nil)

type TransportAdapter struct {
	transportType string
	transportTag  string
	dependencies  []string
	strategy      C.DomainStrategy
	clientSubnet  netip.Prefix
}

func NewTransportAdapter(transportType string, transportTag string, dependencies []string) TransportAdapter {
	return TransportAdapter{
		transportType: transportType,
		transportTag:  transportTag,
		dependencies:  dependencies,
	}
}

func NewTransportAdapterWithLocalOptions(transportType string, transportTag string, localOptions option.LocalDNSServerOptions) TransportAdapter {
	return TransportAdapter{
		transportType: transportType,
		transportTag:  transportTag,
		strategy:      C.DomainStrategy(localOptions.LegacyStrategy),
		clientSubnet:  localOptions.LegacyClientSubnet,
	}
}

func NewTransportAdapterWithRemoteOptions(transportType string, transportTag string, remoteOptions option.RemoteDNSServerOptions) TransportAdapter {
	var dependencies []string
	if remoteOptions.AddressResolver != "" {
		dependencies = []string{remoteOptions.AddressResolver}
	}
	return TransportAdapter{
		transportType: transportType,
		transportTag:  transportTag,
		dependencies:  dependencies,
		strategy:      C.DomainStrategy(remoteOptions.LegacyStrategy),
		clientSubnet:  remoteOptions.LegacyClientSubnet,
	}
}

func (a *TransportAdapter) Type() string {
	return a.transportType
}

func (a *TransportAdapter) Tag() string {
	return a.transportTag
}

func (a *TransportAdapter) Dependencies() []string {
	return a.dependencies
}

func (a *TransportAdapter) LegacyStrategy() C.DomainStrategy {
	return a.strategy
}

func (a *TransportAdapter) LegacyClientSubnet() netip.Prefix {
	return a.clientSubnet
}
