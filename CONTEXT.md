# sing-box-usbip domain glossary

This file names the load-bearing concepts in the USBIP subsystem so design
discussions, ADRs, and architecture reviews share vocabulary. Concepts here
should be reused verbatim in code identifiers and comments.

## USBIP subsystem

**USBIP** — a wire protocol that exports a USB device over TCP. sing-box
implements both server (export) and client (import) roles. Linux uses
kernel `usbip-host` / `vhci_hcd`; Darwin uses an IOUSBHost capture.

**Bus ID** (`busid`) — string identifying a USB device location on the host
(e.g. `1-1.4`). The unit of admission control: the ledger reserves a busid,
not a device handle.

**Export** — a host-owned handle to a single locally attached device that
the server publishes to control subscribers and hands to an import
session. The `Export` interface (`service/usbip/host.go`) is the seam
between platform-specific host code and the platform-neutral ledger.
Once published into a reconcile snapshot, an Export pointer is treated
as immutable (see ADR-0001).

**Export Host** (`ExportHost`) — the platform implementation that
discovers candidate devices, owns the OS-level capture (sysfs bind on
Linux, IOKit open on Darwin), and produces the Export map via
`Reconcile`. One `ExportHost` per server.

**Import Host** (`ImportHost`) — the symmetric platform implementation
on the client side; takes a wire conn and attaches it to the local
USB stack (Linux: `vhci_hcd` attach; Darwin: stub).

**Export Ledger** (`exportLedger` in `service/usbip/export_ledger.go`) —
the per-server admission, lease, and broadcast authority. Owns three
pieces of state under one mutex (the "inventory" lock):

- `exports` — published Export pointers, keyed by busid.
- `busy` — busids with an active import session.
- `leases` — short-lived holds that block admission until the holder
  consumes them or the TTL expires.

Plus broadcast bookkeeping under a separate "fast" lock (sequence,
subscribers, last-broadcast state).

**Reserved State** — the union of busy and unexpired-lease entries for
a given busid. `reservedLocked(busid)` returns this. Every change to
reserved state MUST broadcast a control-frame delta to subscribers —
the `mutateAndBroadcast` / `withInventoryWrite` accessor enforces this
structurally.

**Lease** — a control-channel reservation (`LEASE_REQ` → `IMPORT_EXT`)
that pins a device for one extended client. Identified by `(lease ID,
client nonce)`; pinned to the export's `LeaseIdentity` (registry ID on
Darwin, sysfs identity on Linux) so reconcile-driven swaps invalidate
stale leases.

**Reconcile** — the host operation that diffs the desired set of
matched devices against the currently exported set and produces a
plan (`toAdd`, `toRemove`, `toStale`). Stale entries remain in the
published snapshot (with `state: unavailable`) so subscribers see a
transition instead of a silent removal.

**Stale Export** — an Export the host wanted to drop while it was still
reserved. Stays owned by the host until the holding import session
ends; surfaces to subscribers as `state: unavailable`. Linux marks the
clone; Darwin marks the clone (after ADR-0001).

**Control Subscriber** — a client connected to the control channel
receiving `controlFrameDeviceSnapshot` and `controlFrameDeviceDelta`
frames. "Extended" subscribers (`supportsControlExtensions`) get
deltas; legacy subscribers get `controlFrameChanged` and re-fetch.

## Platform conventions

**Linux Export** (`linuxExport`) — wraps a sysfs `usbip-host` binding.
Identity is the cached descriptor + identity slice; published pointers
are immutable, cloned via `cloneLinuxExport` before stale-marking.

**Darwin Export** (`darwinExport`) — wraps an IOUSBHost capture
(`darwinUSBHostDevice`). Identity is the IOKit `registryID`; published
pointers are immutable, cloned via `cloneDarwinExport` before
stale-marking.

## Decision records

Architecture decisions live in `docs/adr/`. Active records:

- ADR-0001 — Export pointer immutability (the rule that lets the ledger
  read `Snapshot`/`LeaseCheck` outside any lock).
