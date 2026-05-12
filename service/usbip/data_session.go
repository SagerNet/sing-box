package usbip

// DataSession implementations MUST close the channel returned by Done
// when the session terminates for any reason. Err is only valid after
// Done is closed; it returns nil for a clean detach. Close is idempotent
// and safe to call from any goroutine.
type DataSession interface {
	Done() <-chan struct{}
	Err() error
	Close() error
}
