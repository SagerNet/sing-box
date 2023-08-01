package sleep

import (
	"sync"
)

type Manager struct {
	access sync.Mutex
	done   chan struct{}
}

func NewManager() *Manager {
	closedChan := make(chan struct{})
	close(closedChan)
	return &Manager{
		done: closedChan,
	}
}

func (m *Manager) Sleep() {
	m.access.Lock()
	defer m.access.Unlock()
	select {
	case <-m.done:
	default:
		return
	}
	m.done = make(chan struct{})
}

func (m *Manager) Wake() {
	m.access.Lock()
	defer m.access.Unlock()
	select {
	case <-m.done:
	default:
		close(m.done)
	}
}

func (m *Manager) Active() <-chan struct{} {
	return m.done
}
