//go:build !android || !cgo

package jni

func VM() uintptr {
	return 0
}
