//go:build linux || (darwin && cgo)

package usbip

import (
	"context"
	"net"
)

// ExportHost lifecycle: Start → Reconcile* → Close. Reconcile may be
// called many times; FinishImport runs after each data session ends so
// the platform can do post-import cleanup (Linux: write -1 to
// usbip_sockfd; Darwin: release stale-marked captures).
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

type Export interface {
	BusID() string
	Snapshot(ctx context.Context, busy bool) ExportSnapshot
	LeaseCheck(ctx context.Context) (ok bool, reason string)
	DeviceInfo(ctx context.Context) (DeviceInfoTruncated, error)
	NewServerDataSession(ctx context.Context, conn net.Conn) (DataSession, error)
}

// ExportSnapshot Backend and StableID fields are populated
// unconditionally so the caller can build an unavailable record even
// when Entry could not be read.
type ExportSnapshot struct {
	Entry        DeviceEntry
	Backend      string
	StableID     string
	State        string
	StatusReason string
	RawStatus    int
}

type AttachedSession interface {
	DataSession
	Description() string
}
