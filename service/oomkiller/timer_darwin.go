//go:build darwin && cgo

package oomkiller

func (t *adaptiveTimer) notifyPressure() {
	t.releaseMemory()
	t.access.Lock()
	if t.timer == nil {
		t.access.Unlock()
		return
	}
	t.forceMinInterval = true
	t.pendingPressureBaseline = true
	t.access.Unlock()
	t.poll()
}
