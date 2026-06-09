//go:build windows && !amd64 && !arm64

package usbipvhci

func assetVersion() string { return "" }

func assetFiles() []assetFile { return nil }
