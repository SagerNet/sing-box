package daemon

type PlatformHandler interface {
	ServiceStop() error
	ServiceReload() error
	SystemProxyStatus() (*SystemProxyStatus, error)
	SetSystemProxyEnabled(enabled bool) error
	TriggerNativeCrash() error
	WriteDebugMessage(message string)
}
