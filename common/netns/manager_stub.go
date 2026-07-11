//go:build !linux

package netns

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
)

type Manager struct{}

func NewManager(logger logger.ContextLogger, namespaces []option.NetworkNamespace, holderArgs []string) (*Manager, error) {
	if len(namespaces) > 0 {
		return nil, E.New("network namespaces are only supported on Linux")
	}
	return &Manager{}, nil
}

func (m *Manager) Name() string {
	return "netns"
}

func (m *Manager) Start(stage adapter.StartStage) error {
	return nil
}

func (m *Manager) Close() error {
	return nil
}

func (m *Manager) ResolvePath(nameOrPath string) string {
	return nameOrPath
}
