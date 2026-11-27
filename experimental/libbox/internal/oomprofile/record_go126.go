//go:build (darwin || linux || windows) && !go1.27

package oomprofile

type memProfileRecord struct {
	AllocBytes, FreeBytes     int64
	AllocObjects, FreeObjects int64
	Stack                     []uintptr
}

func (r *memProfileRecord) allocBytes() int64 {
	return r.AllocBytes
}

func (r *memProfileRecord) inUseBytes() int64 {
	return r.AllocBytes - r.FreeBytes
}
