#!/usr/bin/env bash

set -euo pipefail

# Go's internal/cpu never detects ARM64 features on windows/arm64 before Go 1.27,
# so crypto/aes, AES-GCM and crypto/sha256 use their generic implementations.
# Backport of https://go.dev/cl/745560 (golang/go#76791).
#
# Remove once the Go release used by the Windows builds includes the fix.

export GOTOOLCHAIN=local

GOROOT="$(go env GOROOT)"
PATCH_FILE="$(cd "$(dirname "$0")" && pwd)/go_windows_arm64_cpu_features.patch"

cd "$GOROOT"
if [[ -f src/internal/cpu/cpu_arm64_windows.go ]]; then
  echo "already patched"
else
  patch --verbose -p1 < "$PATCH_FILE"
fi

CPU_FILES="$(GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go list -f '{{.GoFiles}}' internal/cpu)"
echo "internal/cpu files for windows/arm64: $CPU_FILES"
case "$CPU_FILES" in
*cpu_arm64_windows.go*) ;;
*)
  echo "patch is not effective" >&2
  exit 1
  ;;
esac
