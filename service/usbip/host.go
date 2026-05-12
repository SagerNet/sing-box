//go:build linux || (darwin && cgo)

package usbip

import (
	"context"
	"net"
)

// ExportHost is the server-side seam that owns physical device
// acquisition and the platform-specific bind/unbind dance. Linux backs
// it with sysfs/usbip-host; Darwin backs it with IOKit/IOUSBHost.
//
// Lifecycle:
//
//	Start → Reconcile* → Close
//
// Reconcile may be called many times. The service drives reconcile in
// response to ticks or Events(); the host owns the platform-specific
// enumerate/diff/acquire/release flow internally. FinishImport is
// called after each data session ends so the host can run
// platform-specific post-import cleanup (Linux: write -1 to
// usbip_sockfd; Darwin: release stale-marked captures).
type ExportHost interface {
	Start(ctx context.Context) error
	Close() error
	Reconcile(ctx context.Context, isBusy func(busid string) bool) (snapshot map[string]Export, released []string, changed bool, err error)
	FinishImport(ctx context.Context, busid string) (released bool, err error)
	// Events returns a channel that fires when device topology changes.
	// Returning (nil, nil) means "no native event source; rely on
	// polling". Linux returns (nil, nil); Darwin returns an IOKit-backed
	// channel.
	Events(ctx context.Context) (<-chan struct{}, error)
}

// ImportHost is the client-side seam that owns the data-plane attach.
// Linux backs it with vhci_hcd; Darwin backs it with a virtual XHCI
// controller running in userspace.
type ImportHost interface {
	Start(ctx context.Context) error
	Close() error
	// Attach takes ownership of conn for the lifetime of the import. The
	// returned AttachedSession is a DataSession with a human-readable
	// description for logging.
	Attach(ctx context.Context, info DeviceInfoTruncated, conn net.Conn) (AttachedSession, error)
}

// Export represents a single device the server has acquired and is
// advertising. Implementations are owned by an ExportHost; service code
// drives them through this interface plus a small amount of busy-state
// metadata maintained on ServerService.
type Export interface {
	BusID() string
	// Snapshot returns the per-tick view of this export used to build
	// DeviceInfoV2 broadcasts. The busy flag reflects the service's
	// view of whether a data session is active.
	Snapshot(ctx context.Context, busy bool) ExportSnapshot
	// LeaseCheck reports whether the export is available right now for
	// a new lease. Reason is the human-readable failure cause when ok
	// is false.
	LeaseCheck(ctx context.Context) (ok bool, reason string)
	// DeviceInfo returns the device info to embed in OpRepImport reply.
	DeviceInfo(ctx context.Context) (DeviceInfoTruncated, error)
	// NewServerDataSession runs the per-import data plane against conn.
	NewServerDataSession(ctx context.Context, conn net.Conn) (DataSession, error)
}

// ExportSnapshot is the per-tick view of an Export. The Backend and
// StableID fields are populated unconditionally so the caller can build
// an unavailable record even when Entry could not be read.
type ExportSnapshot struct {
	Entry        DeviceEntry
	Backend      string
	StableID     string
	State        string
	StatusReason string
	RawStatus    int
	// Err is set when Entry could not be read. The caller should emit
	// an unavailable record when Err != nil.
	Err error
}

// AttachedSession is the import-side counterpart of DataSession. Linux
// drives it through the kernel after a handoff; Darwin drives it from
// userspace. Both shapes signal completion via DataSession.Done().
type AttachedSession interface {
	DataSession
	Description() string
}
