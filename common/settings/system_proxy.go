package settings

type SystemProxy interface {
	IsEnabled() bool
	Enable() error
	Disable() error
	Close() error
}
