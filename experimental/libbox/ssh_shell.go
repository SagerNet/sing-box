package libbox

type ShellSession interface {
	MasterFD() int32
	Resize(rows int32, cols int32) error
	Signal(signal int32) error
	WaitExit() (int32, error)
	Close() error
}
