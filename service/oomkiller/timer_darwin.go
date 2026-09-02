//go:build darwin && cgo

package oomkiller

import runtimeDebug "runtime/debug"

func (t *adaptiveTimer) notifyPressure() {
	badCleanup()
	runtimeDebug.FreeOSMemory()
	t.access.Lock()
	t.startLocked()
	t.forceMinInterval = true
	t.pendingPressureBaseline = true
	t.access.Unlock()
	t.poll()
}
