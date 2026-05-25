//go:build linux || (darwin && cgo) || windows

package usbip

// Submit is called from per-endpoint goroutines; the session serializes
// per-endpoint so the engine needs no cross-endpoint lock.
type URBEngine interface {
	Submit(request URBRequest) URBResponse
	AbortEndpoint(endpoint uint8) error
	// Idempotent.
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
