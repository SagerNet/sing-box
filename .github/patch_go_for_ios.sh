#!/usr/bin/env bash

set -euo pipefail

# Go's internal/cpu never detects ARM64 features on GOOS=ios, so crypto/aes,
# AES-GCM and crypto/sha256 use their generic implementations on iOS and tvOS.
# See https://github.com/SagerNet/sing-box/issues/4486
#
# Only the ARMv8.0 cryptographic extensions are asserted; ARMv8.1 atomics and
# SHA-512 are absent on the A8/A9 devices still supported by the deployment targets.
#
# Remove once the Go release used by the Apple library build includes the fix.

export GOTOOLCHAIN=local

GOROOT="$(go env GOROOT)"
PATCH_FILE="$(cd "$(dirname "$0")" && pwd)/go_ios_cpu_features.patch"

cd "$GOROOT"
if [[ -f src/internal/cpu/cpu_arm64_ios.go ]]; then
  echo "already patched"
else
  patch --verbose -p1 < "$PATCH_FILE"
fi

CPU_FILES="$(GOOS=ios GOARCH=arm64 CGO_ENABLED=0 go list -f '{{.GoFiles}}' internal/cpu)"
echo "internal/cpu files for ios/arm64: $CPU_FILES"
case "$CPU_FILES" in
*cpu_arm64_ios.go*) ;;
*)
  echo "patch is not effective" >&2
  exit 1
  ;;
esac
