package usbip

// DataSession is the per-import data-plane lifecycle. Once an import
// handshake succeeds, the server hands the wire connection off to a
// DataSession that runs until the device detaches or the caller closes
// it. Three concrete implementations exist:
//
//   - kernelHandoffSession (Linux): writes the socket fd to usbip_sockfd
//     and lets the kernel drive the data plane; monitors the dup'd fd
//     for hang-up.
//   - darwinServerDataSession (Darwin server): reads USB/IP commands,
//     dispatches them to an IOUSBHost-backed device, writes replies back.
//   - darwinVirtualController (Darwin client): synthesizes an
//     IOUSBHostControllerInterface and translates IOKit traffic into
//     USB/IP frames on the wire.
//
// Implementations MUST close the channel returned by Done when the
// session terminates for any reason. Err is only valid after Done is
// closed; it returns nil for a clean detach. Close is idempotent and
// safe to call from any goroutine.
type DataSession interface {
	Done() <-chan struct{}
	Err() error
	Close() error
}
