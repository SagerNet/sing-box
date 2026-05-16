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
	Reconcile(ctx context.Context, isReserved func(busid string) bool) (snapshot map[string]Export, released []string, err error)
	FinishImport(ctx context.Context, busid string) (released bool, err error)
	// Events returns a coalescing channel that signals topology
	// changes. The channel is closed when Close() is called, which
	// also stops the host's background goroutines. Events MUST be
	// called after Start succeeds. A non-nil error means the host
	// could not subscribe and the server must not continue.
	Events() (<-chan struct{}, error)
}

type ImportHost interface {
	Start(ctx context.Context) error
	Close() error
	// Attach takes ownership of conn for the lifetime of the import.
	Attach(ctx context.Context, info DeviceInfoTruncated, conn net.Conn) (AttachedSession, error)
}

type ExportLeaseIdentity string

// Export is the host-owned handle to a single USB device that the ledger
// publishes to control subscribers and hands to import sessions.
//
// Pointers returned from ExportHost.Reconcile MUST be treated as
// immutable from the ledger's perspective: every field read by
// BusID, Snapshot, LeaseIdentity, LeaseCheck, or DeviceInfo must not
// change after the Export is published into the snapshot map. The
// ledger calls these methods outside any lock — concurrent mutation is
// a data race. Hosts that need to change such state (mark stale, swap
// underlying device handle, …) MUST publish a fresh Export pointer by
// cloning, mutating the clone, then swapping it into the host's
// committed map under the host's own lock. See cloneLinuxExport and
// docs/adr/0001-export-pointer-immutability.md for the canonical
// pattern; applyStaleClones enforces the rule for stale transitions.
type Export interface {
	BusID() string
	Snapshot(ctx context.Context, busy bool) ExportSnapshot
	LeaseIdentity() ExportLeaseIdentity
	LeaseCheck(ctx context.Context) (ok bool, reason string)
	DeviceInfo(ctx context.Context) (DeviceInfoTruncated, error)
	NewServerDataSession(ctx context.Context, conn net.Conn) (DataSession, error)
}

// applyStaleClones honours the Export immutability contract for
// stale-mark transitions: for every busid in toStale that exists in
// committed, it allocates a clone, runs mark on the clone, and writes
// the clone back. The previously published pointer is never mutated.
// Callers that consumed the original pointer continue to observe its
// pre-stale snapshot until they next refresh from committed.
func applyStaleClones[T any](committed map[string]*T, toStale []string, clone func(*T) *T, mark func(*T)) {
	for _, busid := range toStale {
		exp, found := committed[busid]
		if !found {
			continue
		}
		cloned := clone(exp)
		mark(cloned)
		committed[busid] = cloned
	}
}

// ExportSnapshot Backend and StableID fields are populated
// unconditionally. Unavailable snapshots keep a cached Entry,
// including BusID, so the caller can broadcast a state transition
// instead of removing the device outright; snapshots without a BusID
// are treated as non-broadcastable. ExportHost.Reconcile returns
// stale entries in its snapshot for the same reason — the ledger
// filters non-broadcastable snapshots in one place, not the host.
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
