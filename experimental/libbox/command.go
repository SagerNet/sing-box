//go:build darwin

package libbox

const (
	CommandLog int32 = iota
	CommandStatus
	CommandServiceStop
	CommandServiceReload
	CommandCloseConnections
)
