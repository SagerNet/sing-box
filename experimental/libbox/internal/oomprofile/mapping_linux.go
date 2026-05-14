//go:build linux

package oomprofile

import "os"

func (b *profileBuilder) readMapping() {
	data, _ := os.ReadFile("/proc/self/maps")
	stdParseProcSelfMaps(data, func(lo, hi, offset uint64, file, buildID string) {
		b.addMappingEntry(lo, hi, offset, file, buildID, false)
	})
	if len(b.mem) == 0 {
		b.addMappingEntry(0, 0, 0, "", "", true)
	}
}
