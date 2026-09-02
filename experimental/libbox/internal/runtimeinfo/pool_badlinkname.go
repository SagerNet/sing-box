//go:build badlinkname

package runtimeinfo

import (
	"reflect"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/sagernet/sing/common/buf"
)

// Mirrors sync.Pool, sync.poolLocal and sync.poolChainElt of the pinned Go toolchain.

type syncPool struct {
	local      unsafe.Pointer
	localSize  uintptr
	victim     unsafe.Pointer
	victimSize uintptr
	_          func() any
}

type poolLocal struct {
	private    [2]unsafe.Pointer
	sharedHead unsafe.Pointer
	_          unsafe.Pointer
	_          [128 - 4*unsafe.Sizeof(uintptr(0))]byte
}

type poolChainElt struct {
	headTail uint64
	_        []unsafe.Pointer
	_        unsafe.Pointer
	prev     unsafe.Pointer
}

func collectBufferPools() []PoolReport {
	allocator := reflect.ValueOf(buf.DefaultAllocator)
	if allocator.Kind() != reflect.Pointer {
		return nil
	}
	pools := allocator.Elem().FieldByName("buffers")
	if !pools.IsValid() || pools.Kind() != reflect.Array || pools.Type().Elem() != reflect.TypeFor[sync.Pool]() {
		return nil
	}
	if unsafe.Sizeof(syncPool{}) != unsafe.Sizeof(sync.Pool{}) || unsafe.Sizeof(poolLocal{}) != 128 {
		return nil
	}
	result := make([]PoolReport, 0, pools.Len())
	for i := range pools.Len() {
		pool := (*syncPool)(pools.Index(i).Addr().UnsafePointer())
		size := min(1<<(6+i), buf.MaxPooledBufferSize)
		report := PoolReport{
			Size:   size,
			Cached: countPoolLocals(pool.local, pool.localSize),
			Victim: countPoolLocals(pool.victim, pool.victimSize),
		}
		report.Bytes = uint64(report.Cached+report.Victim) * uint64(size)
		result = append(result, report)
	}
	return result
}

func countPoolLocals(locals unsafe.Pointer, count uintptr) int {
	if locals == nil {
		return 0
	}
	var total int
	for i := range count {
		local := (*poolLocal)(unsafe.Add(locals, i*unsafe.Sizeof(poolLocal{})))
		if local.private[0] != nil {
			total++
		}
		for element := (*poolChainElt)(atomic.LoadPointer(&local.sharedHead)); element != nil; element = (*poolChainElt)(atomic.LoadPointer(&element.prev)) {
			headTail := atomic.LoadUint64(&element.headTail)
			total += int(uint32(headTail>>32) - uint32(headTail))
		}
	}
	return total
}
