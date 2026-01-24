package daemon

type PlatformHandler interface {
	ServiceStop() error
	ServiceReload() error
	SystemProxyStatus() (*SystemProxyStatus, error)
	SetSystemProxyEnabled(enabled bool) error
	WriteDebugMessage(message string)
}
