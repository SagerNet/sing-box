package log

type PlatformWriter interface {
	DisableColors() bool
	WriteMessage(level Level, message string)
}
