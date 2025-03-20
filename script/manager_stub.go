//go:build !with_script

package script

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ adapter.ScriptManager = (*Manager)(nil)

type Manager struct{}

func NewManager(ctx context.Context, logFactory log.Factory, scripts []option.Script) (*Manager, error) {
	if len(scripts) > 0 {
		return nil, E.New(`script is not included in this build, rebuild with -tags with_script`)
	}
	return (*Manager)(nil), nil
}

func (m *Manager) Start(stage adapter.StartStage) error {
	return nil
}

func (m *Manager) Close() error {
	return nil
}

func (m *Manager) Scripts() []adapter.Script {
	return nil
}

func (m *Manager) Script(name string) (adapter.Script, bool) {
	return nil, false
}

func (m *Manager) SurgeCache() *adapter.SurgeInMemoryCache {
	return nil
}
