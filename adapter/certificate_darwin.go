//go:build darwin && cgo

package adapter

import "unsafe"

type AppleAnchors interface {
	Retain() AppleAnchors
	Release()
	// Ref returns the underlying CFArrayRef, or nil if the anchor set is empty.
	Ref() unsafe.Pointer
}

type AppleCertificateStore interface {
	CertificateStore
	AppleAnchors() AppleAnchors
}
