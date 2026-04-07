//go:build darwin

package oomprofile

import (
	"encoding/binary"
	"os"
	"unsafe"
	_ "unsafe"
)

func isExecutable(protection int32) bool {
	return (protection&_VM_PROT_EXECUTE) != 0 && (protection&_VM_PROT_READ) != 0
}

func (b *profileBuilder) readMapping() {
	if !machVMInfo(b.addMapping) {
		b.addMappingEntry(0, 0, 0, "", "", true)
	}
}

func machVMInfo(addMapping func(lo uint64, hi uint64, off uint64, file string, buildID string)) bool {
	added := false
	addr := uint64(0x1)
	for {
		var regionSize uint64
		var info machVMRegionBasicInfoData
		kr := machVMRegion(&addr, &regionSize, unsafe.Pointer(&info))
		if kr != 0 {
			if kr == _MACH_SEND_INVALID_DEST {
				return true
			}
			return added
		}
		if isExecutable(info.Protection) {
			addMapping(addr, addr+regionSize, binary.LittleEndian.Uint64(info.Offset[:]), regionFilename(addr), "")
			added = true
		}
		addr += regionSize
	}
}

func regionFilename(address uint64) string {
	buf := make([]byte, _MAXPATHLEN)
	n := procRegionFilename(os.Getpid(), address, unsafe.SliceData(buf), int64(cap(buf)))
	if n == 0 {
		return ""
	}
	return string(buf[:n])
}

//go:linkname machVMRegion runtime/pprof.mach_vm_region
func machVMRegion(address *uint64, regionSize *uint64, info unsafe.Pointer) int32

//go:linkname procRegionFilename runtime/pprof.proc_regionfilename
func procRegionFilename(pid int, address uint64, buf *byte, buflen int64) int32
