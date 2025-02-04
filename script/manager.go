package script

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/task"
)

var _ adapter.ScriptManager = (*Manager)(nil)

type Manager struct {
	ctx          context.Context
	logger       logger.ContextLogger
	scripts      []adapter.Script
	scriptByName map[string]adapter.Script
	surgeCache   *adapter.SurgeInMemoryCache
}

func NewManager(ctx context.Context, logFactory log.Factory, scripts []option.Script) (*Manager, error) {
	manager := &Manager{
		ctx:          ctx,
		logger:       logFactory.NewLogger("script"),
		scriptByName: make(map[string]adapter.Script),
	}
	for _, scriptOptions := range scripts {
		script, err := NewScript(ctx, logFactory.NewLogger(F.ToString("script/", scriptOptions.Type, "[", scriptOptions.Tag, "]")), scriptOptions)
		if err != nil {
			return nil, E.Cause(err, "initialize script: ", scriptOptions.Tag)
		}
		manager.scripts = append(manager.scripts, script)
		manager.scriptByName[scriptOptions.Tag] = script
	}
	return manager, nil
}

func (m *Manager) Start(stage adapter.StartStage) error {
	monitor := taskmonitor.New(m.logger, C.StartTimeout)
	switch stage {
	case adapter.StartStateStart:
		var cacheContext *adapter.HTTPStartContext
		if len(m.scripts) > 0 {
			monitor.Start("initialize rule-set")
			cacheContext = adapter.NewHTTPStartContext(m.ctx)
			var scriptStartGroup task.Group
			for _, script := range m.scripts {
				scriptInPlace := script
				scriptStartGroup.Append0(func(ctx context.Context) error {
					err := scriptInPlace.StartContext(ctx, cacheContext)
					if err != nil {
						return E.Cause(err, "initialize script/", scriptInPlace.Type(), "[", scriptInPlace.Tag(), "]")
					}
					return nil
				})
			}
			scriptStartGroup.Concurrency(5)
			scriptStartGroup.FastFail()
			err := scriptStartGroup.Run(m.ctx)
			monitor.Finish()
			if err != nil {
				return err
			}
		}
		if cacheContext != nil {
			cacheContext.Close()
		}
	case adapter.StartStatePostStart:
		for _, script := range m.scripts {
			monitor.Start(F.ToString("post start script/", script.Type(), "[", script.Tag(), "]"))
			err := script.PostStart()
			monitor.Finish()
			if err != nil {
				return E.Cause(err, "post start script/", script.Type(), "[", script.Tag(), "]")
			}
		}
	}
	return nil
}

func (m *Manager) Close() error {
	monitor := taskmonitor.New(m.logger, C.StopTimeout)
	var err error
	for _, script := range m.scripts {
		monitor.Start(F.ToString("close start script/", script.Type(), "[", script.Tag(), "]"))
		err = E.Append(err, script.Close(), func(err error) error {
			return E.Cause(err, "close script/", script.Type(), "[", script.Tag(), "]")
		})
		monitor.Finish()
	}
	return err
}

func (m *Manager) Scripts() []adapter.Script {
	return m.scripts
}

func (m *Manager) Script(name string) (adapter.Script, bool) {
	script, loaded := m.scriptByName[name]
	return script, loaded
}

func (m *Manager) SurgeCache() *adapter.SurgeInMemoryCache {
	if m.surgeCache == nil {
		m.surgeCache = &adapter.SurgeInMemoryCache{
			Data: make(map[string]string),
		}
	}
	return m.surgeCache
}
