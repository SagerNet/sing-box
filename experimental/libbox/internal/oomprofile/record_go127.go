//go:build (darwin || linux || windows) && go1.27

package oomprofile

type memProfileRecord struct {
	ObjectSize                int64
	AllocObjects, FreeObjects int64
	Stack                     []uintptr
}

func (r *memProfileRecord) allocBytes() int64 {
	return r.AllocObjects * r.ObjectSize
}

func (r *memProfileRecord) inUseBytes() int64 {
	return r.inUseObjects() * r.ObjectSize
}
