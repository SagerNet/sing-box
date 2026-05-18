//go:build linux || (darwin && cgo)

package usbip

// URBEngine executes USB Request Blocks against an already-claimed
// device. The session layer (session_userspace.go) handles framing,
// per-endpoint ordering, and unlink bookkeeping; the engine performs
// the per-URB I/O and per-endpoint aborts only.
//
// Submit is called from per-endpoint goroutines; the session never
// issues two Submits concurrently for the same endpoint, so the engine
// does not need its own cross-endpoint serialization.
type URBEngine interface {
	Submit(request URBRequest) URBResponse
	// AbortEndpoint cancels all in-flight submits on the given raw
	// endpoint address (direction bit included). It is invoked once per
	// pending sequence at CMD_UNLINK time and once per active endpoint
	// at session shutdown.
	AbortEndpoint(endpoint uint8) error
	// Close releases engine-owned resources. For engines that own the
	// underlying device handle (e.g. Windows VBoxUSB), this releases it.
	// For engines where the host manages the device handle separately
	// (e.g. Darwin IOUSBHost capture), Close may be a no-op. Idempotent.
	Close() error
}

// URBRequest carries one decoded CMD_SUBMIT plus session-owned buffers.
// Buffer holds the OUT payload on entry, or a pre-allocated zero buffer
// for IN transfers. IsoPackets is pre-cloned from the wire command so
// the engine may overwrite descriptors in place during iso completion.
type URBRequest struct {
	Command    SubmitCommand
	Endpoint   uint8
	Buffer     []byte
	IsoPackets []IsoPacketDescriptor
}

// URBResponse is the engine's verdict on one URB. Status follows USBIP
// convention (negated errno, 0 on success). ActualLength is the number
// of payload bytes valid in Buffer. Error is engine-internal failure
// distinct from a USB-level error: on Error the session emits
// Status = usbipStatusEIO and logs at Debug.
type URBResponse struct {
	Status       int32
	ActualLength int32
	Buffer       []byte
	IsoPackets   []IsoPacketDescriptor
	Error        error
}
