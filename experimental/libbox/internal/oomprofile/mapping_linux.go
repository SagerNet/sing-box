//go:build linux

package oomprofile

import "os"

func (b *profileBuilder) readMapping() {
	data, _ := os.ReadFile("/proc/self/maps")
	stdParseProcSelfMaps(data, b.addMapping)
	if len(b.mem) == 0 {
		b.addMappingEntry(0, 0, 0, "", "", true)
	}
}
