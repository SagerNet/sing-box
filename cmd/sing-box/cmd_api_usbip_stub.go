//go:build !with_usbip || !(linux || (darwin && cgo) || windows)

package main

import (
	E "github.com/sagernet/sing/common/exceptions"
)

func runAPIUsbipDeviceList() error {
	return errUsbipLocalNotIncluded()
}

func runAPIUsbipDeviceShow(_ string) error {
	return errUsbipLocalNotIncluded()
}

func runAPIUsbipShare(_ []string) error {
	return errUsbipLocalNotIncluded()
}

func errUsbipLocalNotIncluded() error {
	return E.New(`USB/IP is not included in this build, rebuild with -tags with_usbip (supported on Linux, Windows, and macOS with CGO)`)
}
