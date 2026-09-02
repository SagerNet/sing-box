//go:build badlinkname

package runtimeinfo

import (
	"bytes"
	"cmp"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"unsafe"
)

// Mirrors runtime.g of the pinned Go toolchain up to goid; the offsets of gopc and startpc are
// discovered by probeGoroutine.

type runtimeG struct {
	stackLo      uintptr
	stackHi      uintptr
	_            [2]uintptr
	_            [3]unsafe.Pointer
	_            [6]uintptr
	_            [4]uintptr
	_            unsafe.Pointer
	atomicstatus uint32
	_            uint32
	goid         uint64
}

//go:linkname allgs runtime.allgs
var allgs []*runtimeG

const probeScanBytes = 512

var (
	gopcOffset    uintptr
	startpcOffset uintptr
	probeDone     chan bool
)

func (gp *runtimeG) word(offset uintptr) uintptr {
	return *(*uintptr)(unsafe.Add(unsafe.Pointer(gp), offset))
}

func (gp *runtimeG) gopc() uintptr {
	return gp.word(gopcOffset)
}

func (gp *runtimeG) startpc() uintptr {
	return gp.word(startpcOffset)
}

const (
	statusDead = 6
	statusScan = 0x1000
)

var statusNames = map[uint32]string{
	0: "idle",
	1: "runnable",
	2: "running",
	3: "syscall",
	4: "waiting",
	6: "dead",
	8: "copystack",
	9: "preempted",
}

var (
	layoutOnce     sync.Once
	layoutVerified bool
)

func probeLayout() {
	probeDone = make(chan bool, 1)
	go probeGoroutine()
	layoutVerified = <-probeDone
}

func probeGoroutine() {
	id := currentGoroutineID()
	var self *runtimeG
	for _, gp := range allgs {
		if gp.goid == id {
			self = gp
			break
		}
	}
	if self == nil {
		probeDone <- false
		return
	}
	creatorName := functionName(reflect.ValueOf(probeLayout).Pointer())
	selfName := functionName(reflect.ValueOf(probeGoroutine).Pointer())
	goidOffset := unsafe.Offsetof(self.goid)
	for offset := goidOffset + 8; offset < goidOffset+probeScanBytes; offset += 8 {
		name := functionName(self.word(offset))
		if gopcOffset == 0 && name == creatorName {
			gopcOffset = offset
		} else if startpcOffset == 0 && name == selfName {
			startpcOffset = offset
		}
	}
	probeDone <- gopcOffset != 0 && startpcOffset != 0
}

func functionName(pc uintptr) string {
	function := runtime.FuncForPC(pc)
	if function == nil {
		return "unknown"
	}
	return function.Name()
}

func currentGoroutineID() uint64 {
	var stackBuffer [64]byte
	n := runtime.Stack(stackBuffer[:], false)
	fields := bytes.Fields(stackBuffer[:n])
	if len(fields) < 2 {
		return 0
	}
	id, _ := strconv.ParseUint(string(fields[1]), 10, 64)
	return id
}

func collectGoroutines() *GoroutineReport {
	layoutOnce.Do(probeLayout)
	if !layoutVerified {
		return nil
	}
	report := &GoroutineReport{ByStatus: make(map[string]int)}
	groups := make(map[string]*GoroutineGroup)
	for _, gp := range allgs {
		status := atomic.LoadUint32(&gp.atomicstatus) &^ statusScan
		stackSize := uint64(gp.stackHi - gp.stackLo)
		if status == statusDead {
			report.Dead++
			if gp.stackLo != 0 {
				report.DeadStackBytes += stackSize
			}
			continue
		}
		report.Total++
		report.StackBytes += stackSize
		statusName, known := statusNames[status]
		if !known {
			statusName = strconv.Itoa(int(status))
		}
		report.ByStatus[statusName]++
		name := functionName(gp.startpc())
		group, found := groups[name]
		if !found {
			group = &GoroutineGroup{Function: name, CreatedBy: functionName(gp.gopc()), MinStackBytes: stackSize}
			groups[name] = group
		}
		group.Count++
		group.StackBytes += stackSize
		group.MaxStackBytes = max(group.MaxStackBytes, stackSize)
		group.MinStackBytes = min(group.MinStackBytes, stackSize)
	}
	report.ByFunction = make([]GoroutineGroup, 0, len(groups))
	for _, group := range groups {
		report.ByFunction = append(report.ByFunction, *group)
	}
	slices.SortFunc(report.ByFunction, func(a, b GoroutineGroup) int {
		return cmp.Compare(b.StackBytes, a.StackBytes)
	})
	return report
}
