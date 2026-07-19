# Dart patches

This fork carries one intentionally small downstream patch:

- `allow-http-clash-api-delay.patch` lets the Clash API pass an `http://`
  latency-test URL to the existing URLTest implementation instead of replacing
  it with sing-box's HTTPS default.

The `Sync upstream with Dart patches` workflow merges `SagerNet/sing-box`'s
`stable` branch, verifies that this patch is still applied, compiles the Clash
API package, and cross-builds the Windows amd64 binary before pushing the
updated branch. If upstream changes make the patch ambiguous, the workflow
fails without pushing.
