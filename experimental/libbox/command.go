package libbox

const (
	CommandLog int32 = iota
	CommandStatus
	CommandServiceReload
	CommandServiceClose
	CommandCloseConnections
	CommandGroup
	CommandSelectOutbound
	CommandURLTest
	CommandGroupExpand
	CommandClashMode
	CommandSetClashMode
	CommandGetSystemProxyStatus
	CommandSetSystemProxyEnabled
	CommandConnections
	CommandCloseConnection
	CommandGetDeprecatedNotes
)
