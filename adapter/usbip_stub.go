//go:build !with_usbip || !(linux || (darwin && cgo) || windows)

package adapter

type USBIPDynamicServer interface {
	usbipNotIncluded()
}
