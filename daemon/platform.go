package daemon

import "context"

type PlatformHandler interface {
	WriteDebugMessage(message string)
	ConnectSSHAgent() (int32, error)
}

type ManagedHandler interface {
	ServiceStop() error
	ServiceReload(ctx context.Context) error
	SystemProxyStatus() (*SystemProxyStatus, error)
	SetSystemProxyEnabled(enabled bool) error
	TriggerNativeCrash() error
}
