package tuic

const (
	Version = 5
)

const (
	CommandAuthenticate = iota
	CommandConnect
	CommandPacket
	CommandDissociate
	CommandHeartbeat
)

const AuthenticateLen = 2 + 16 + 32
