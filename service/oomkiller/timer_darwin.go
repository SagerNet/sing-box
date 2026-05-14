//go:build darwin && cgo

package oomkiller

func (t *adaptiveTimer) notifyPressure() {
	t.access.Lock()
	t.startLocked()
	t.forceMinInterval = true
	t.pendingPressureBaseline = true
	t.access.Unlock()
	t.poll()
}
