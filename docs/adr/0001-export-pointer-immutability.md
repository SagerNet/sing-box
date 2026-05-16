# ADR-0001 — Export pointer immutability

- Status: Accepted
- Date: 2026-05-16
- Scope: `service/usbip/host.go`, `service/usbip/host_linux.go`,
  `service/usbip/host_darwin.go`, `service/usbip/export_ledger.go`

## Context

The `Export` interface (`service/usbip/host.go`) is the seam between
platform `ExportHost` implementations and the platform-neutral
`exportLedger`. The ledger publishes Export pointers to control
subscribers and import sessions, and it calls `Snapshot`,
`LeaseCheck`, `LeaseIdentity`, and `DeviceInfo` on those pointers
**outside** any lock (`export_ledger.go` ~ lines 190, 274, 377, 592).
Holding the inventory lock across these calls is not viable because
they may perform syscalls (Linux re-reads `usbip_status`; Darwin
re-reads IOKit state).

Two implementation paths existed:

1. Each host carries a per-Export lock and `Snapshot` etc. acquire it.
   Cost: every method on every Export takes a lock. The lock has to
   live in the platform struct (the interface is read-only). Three
   call sites in the ledger × N hosts. Easy to forget on a new host.
2. Hosts treat published Export pointers as immutable. Mutation is
   "clone the published pointer, mutate the clone, swap it into the
   host's committed map under the host's lock". The ledger's
   unlocked reads are safe because the reader holds a pointer that
   nobody mutates.

Linux already followed pattern 2 by convention (`cloneLinuxExport` +
the `committed` map swap in `Reconcile`). Darwin did not — its
`Reconcile` set `exp.stale = true` and `exp.pendingRegistryID = …`
on the same `*darwinExport` value the ledger had already received
(`host_darwin.go:177-180`). Under `go test -race` this is a clean
data race; in production it can publish an inconsistent tuple
(`stale=true`, `pendingRegistryID=0`) if writes reorder.

## Decision

Pattern 2 is the contract. Published Export pointers are immutable
from the ledger's perspective. Hosts that need to change Export
state MUST clone, mutate the clone, then swap the clone into their
committed map under their own lock. The previously-handed-out pointer
is never written to.

The contract is documented on the `Export` interface declaration
itself. A shared helper `applyStaleClones` (`host.go`) enforces the
clone-then-mark rule for the common "mark a busid stale" transition;
both Linux and Darwin reconcile flows use it.

## Consequences

- The ledger reads `Snapshot/LeaseCheck/…` outside its inventory lock
  without per-Export locking and without races. This keeps the
  inventory lock fast and avoids re-entering hosts under it.
- Hosts pay a small allocation per stale/replace transition (one
  struct value copy plus a `slices.Clone` of any embedded slice).
  Reconcile is not on the hot path.
- New host implementations (Windows, FreeBSD, …) inherit the rule
  via the interface comment, the `applyStaleClones` helper, and this
  ADR. The race detector catches violations on normal test runs
  once `go test -race` is part of CI.
- The contract does not cover external state hidden behind the
  Export (the IOKit handle, the kernel binding). Hosts continue to
  manage that under their own locks; the rule is specifically about
  fields read through the `Export` interface methods.

## References

- `service/usbip/host.go` — interface doc and `applyStaleClones`.
- `service/usbip/host_linux.go` — `cloneLinuxExport`, `committed`
  map pattern in `Reconcile`.
- `service/usbip/host_darwin.go` — `cloneDarwinExport`, mirrored
  `Reconcile` after this ADR.
