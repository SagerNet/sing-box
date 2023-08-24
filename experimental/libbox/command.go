package libbox

const (
	CommandLog int32 = iota
	CommandStatus
	CommandServiceReload
	CommandCloseConnections
	CommandGroup
	CommandSelectOutbound
	CommandURLTest
	CommandGroupExpand
	CommandClashMode
	CommandSetClashMode
)
