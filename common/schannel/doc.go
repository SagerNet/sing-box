// Package schannel wraps the Windows Schannel security provider (SSPI) for
// client-side TLS. The public API is implemented on Windows only; on other
// platforms the package is empty and intended for transitive imports from
// build-tagged callers.
package schannel
