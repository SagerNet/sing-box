//go:build linux || (darwin && cgo) || windows

package usbip

import (
	"context"
	"net"
)

// ExportHost lifecycle: Start → Reconcile* → Close. Callers must apply
// the Reconcile snapshot and released list even when err != nil.
type ExportHost interface {
	Start() error
	Close() error
	Reconcile(isReserved func(busid string) bool) (snapshot map[string]Export, released []string, err error)
	FinishImport(busid string) (released bool, err error)
	// Events MUST be called after Start succeeds. The channel closes when
	// Close() runs.
	Events() (<-chan struct{}, error)
}

type ImportHost interface {
	Start() error
	Close() error
	// Attach takes ownership of conn for the lifetime of the import.
	Attach(ctx context.Context, info DeviceInfoTruncated, conn net.Conn) (AttachedSession, error)
}

type ExportLeaseIdentity string

// Export pointers handed back from Reconcile are immutable from the
// ledger's perspective: the ledger calls the methods below outside any
// lock. Hosts that need to mutate must clone, mutate the clone, then
// swap it into their committed map under the host's own lock.
type Export interface {
	BusID() string
	Snapshot(busy bool) ExportSnapshot
	LeaseIdentity() ExportLeaseIdentity
	LeaseCheck() (ok bool, reason string)
	DeviceInfo() (DeviceInfoTruncated, error)
	NewServerDataSession(ctx context.Context, conn net.Conn) (DataSession, error)
}

// ExportSnapshot entries with an empty BusID are filtered out by the
// ledger; hosts may keep stale snapshots so state transitions broadcast.
type ExportSnapshot struct {
	Entry        DeviceEntry
	Backend      string
	StableID     string
	State        string
	StatusReason string
	RawStatus    int
}

// DataSession implementations MUST close Done when the session
// terminates. Err is only valid after Done. Start and Close are
// idempotent; Close before Start releases all resources.
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
