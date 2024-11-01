package outbound

import (
	"github.com/sagernet/sing-box/option"
)

type Adapter struct {
	protocol     string
	network      []string
	tag          string
	dependencies []string
}

func NewAdapter(protocol string, network []string, tag string, dependencies []string) Adapter {
	return Adapter{
		protocol:     protocol,
		network:      network,
		tag:          tag,
		dependencies: dependencies,
	}
}

func NewAdapterWithDialerOptions(protocol string, network []string, tag string, dialOptions option.DialerOptions) Adapter {
	var dependencies []string
	if dialOptions.Detour != "" {
		dependencies = []string{dialOptions.Detour}
	}
	return NewAdapter(protocol, network, tag, dependencies)
}

func (a *Adapter) Type() string {
	return a.protocol
}

func (a *Adapter) Tag() string {
	return a.tag
}

func (a *Adapter) Network() []string {
	return a.network
}

func (a *Adapter) Dependencies() []string {
	return a.dependencies
}
