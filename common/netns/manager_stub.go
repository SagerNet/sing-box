//go:build !linux

package netns

import (
	E "github.com/sagernet/sing/common/exceptions"
)

type holder struct{}

func (m *Manager) start() error {
	if len(m.namespaces) > 0 {
		return E.New("network namespaces are only supported on Linux")
	}
	return nil
}

func (m *Manager) close() error {
	return nil
}
