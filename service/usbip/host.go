//go:build linux || (darwin && cgo)

package usbip

import (
	"context"
	"net"
)

// ExportHost lifecycle: Start → Reconcile* → Close. Reconcile may be
// called many times; it returns the committed post-reconcile export
// state, and callers must apply snapshot/released even when err != nil.
// FinishImport runs after each data session ends so the platform can do
// post-import cleanup (Linux: write -1 to usbip_sockfd; Darwin:
// release stale-marked captures).
type ExportHost interface {
	Start(ctx context.Context) error
	Close() error
	Reconcile(ctx context.Context, isBusy func(busid string) bool) (snapshot map[string]Export, released []string, err error)
	FinishImport(ctx context.Context, busid string) (released bool, err error)
	// Events returning (nil, nil) means "no native event source; rely
	// on polling".
	Events(ctx context.Context) (<-chan struct{}, error)
}

type ImportHost interface {
	Start(ctx context.Context) error
	Close() error
	// Attach takes ownership of conn for the lifetime of the import.
	Attach(ctx context.Context, info DeviceInfoTruncated, conn net.Conn) (AttachedSession, error)
}

type ExportLeaseIdentity string

type Export interface {
	BusID() string
	Snapshot(ctx context.Context, busy bool) ExportSnapshot
	LeaseIdentity() ExportLeaseIdentity
	LeaseCheck(ctx context.Context) (ok bool, reason string)
	DeviceInfo(ctx context.Context) (DeviceInfoTruncated, error)
	NewServerDataSession(ctx context.Context, conn net.Conn) (DataSession, error)
}

// ExportSnapshot Backend and StableID fields are populated
// unconditionally. Unavailable snapshots should keep a cached Entry,
// including BusID, so the caller can broadcast a state transition
// instead of removing the device outright; snapshots without a BusID
// are treated as non-broadcastable.
type ExportSnapshot struct {
	Entry        DeviceEntry
	Backend      string
	StableID     string
	State        string
	StatusReason string
	RawStatus    int
}

// DataSession implementations MUST close the channel returned by Done
// when the session terminates for any reason. Err is only valid after
// Done is closed; it returns nil for a clean detach. Close is idempotent
// and safe to call from any goroutine.
//
// Start transfers the session from "prepared" to "running". The session
// is returned in the prepared state with the userspace conn still alive
// so the server can send OP_REP_IMPORT before the kernel takes wire
// ownership. Start is idempotent. Close before Start releases all
// resources, including the userspace conn.
type DataSession interface {
	Done() <-chan struct{}
	Err() error
	Start() error
	Close() error
}

type AttachedSession interface {
	DataSession
	Description() string
}
