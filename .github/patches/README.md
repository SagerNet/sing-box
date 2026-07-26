# Dart patches

This fork carries the Dart Smart outbound and one intentionally small
downstream patch:

- `allow-http-clash-api-delay.patch` lets the Clash API pass an `http://`
  latency-test URL to the existing URLTest implementation instead of replacing
  it with sing-box's HTTPS default.
- The Dart Smart outbound publishes a bounded, privacy-preserving incremental
  dial feed at `GET /dart/dial-feedback?since=<sequence>`. The feed contains
  only sequence, Smart group, outbound tag, transport network, success, rounded
  duration in milliseconds, timestamp in Unix milliseconds, a classified
  error, and a strict signal name. It never contains a destination or raw error
  string.

The authenticated endpoint returns
`{ "instance": "<128-bit random hex>", "sequence": N, "events": [...] }` and
sets `Cache-Control: no-store`. The process-scoped instance changes across core
restarts so clients can safely reset an otherwise ambiguous sequence cursor. It
uses independent bounded windows for the newest 256 detailed events and newest
256 legacy-compatible outcomes; both share the same global sequence. A stale
cursor receives the retained suffix, while a current cursor receives an empty
array. Failure classes are restricted to `canceled`, `network`, `soft-fail`,
`timeout`, and `unknown`.

Clients can opt in to the full staged feed with `signals=1`. Its `signal` field
is restricted to `tcp`, `udp`, `handshake`, and `first-byte`. Requests without
the opt-in retain the original event count and outcome semantics: provisional
TCP success and first-byte observations stay hidden, while the top-level cursor
still advances globally. This keeps older GUI versions compatible without
replaying hidden events. Detailed `handshake` duration measures only the first
write; its legacy projection preserves the original total dial plus first-write
duration. `tcp`, `udp`, and `first-byte` durations measure their respective
stages.

The `Sync upstream with Dart patches` workflow merges `SagerNet/sing-box`'s
`stable` branch, verifies that the patch and Dart Smart feedback API are still
present, tests the Smart and Clash API packages, and cross-builds the Windows
amd64 binary before pushing the updated branch. If upstream changes make the
patch ambiguous, the workflow fails without pushing.

The `Release Dart patched core` workflow checks the latest stable upstream
release each day. For every new upstream version it creates a matching
`v<version>-dart.1` source tag, verifies the patch, and publishes a Windows
amd64 ZIP plus `SHA256SUMS.txt`. Existing releases are left untouched, and a
manual run can target a specific stable upstream tag.
