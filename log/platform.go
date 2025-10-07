package log

type PlatformWriter interface {
	WriteMessage(level Level, message string)
}
