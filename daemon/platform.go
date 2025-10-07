package daemon

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/option"
)

type PlatformHandler interface {
	ServiceStop() error
	ServiceReload() error
	SystemProxyStatus() (*SystemProxyStatus, error)
	SetSystemProxyEnabled(enabled bool) error
	WriteDebugMessage(message string)
}

type PlatformInterface interface {
	adapter.PlatformInterface

	UsePlatformLocalDNSTransport() bool
	LocalDNSTransport() dns.TransportConstructorFunc[option.LocalDNSServerOptions]
}
