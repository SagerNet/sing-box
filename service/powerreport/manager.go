package powerreport

import (
	"sync"
	"sync/atomic"
)

type Manager struct {
	access   sync.Mutex
	recorder atomic.Pointer[Recorder]
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Start(options Options) error {
	m.access.Lock()
	defer m.access.Unlock()
	if m.recorder.Load() != nil {
		return nil
	}
	recorder := NewRecorder(options)
	err := recorder.Start()
	if err != nil {
		return err
	}
	m.recorder.Store(recorder)
	return nil
}

func (m *Manager) Close() error {
	m.access.Lock()
	defer m.access.Unlock()
	recorder := m.recorder.Swap(nil)
	if recorder == nil {
		return nil
	}
	return recorder.Close()
}

func (m *Manager) Recorder() *Recorder {
	return m.recorder.Load()
}
