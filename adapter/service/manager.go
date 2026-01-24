package service

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

var _ adapter.ServiceManager = (*Manager)(nil)

type Manager struct {
	logger       log.ContextLogger
	registry     adapter.ServiceRegistry
	access       sync.Mutex
	started      bool
	stage        adapter.StartStage
	services     []adapter.Service
	serviceByTag map[string]adapter.Service
}

func NewManager(logger log.ContextLogger, registry adapter.ServiceRegistry) *Manager {
	return &Manager{
		logger:       logger,
		registry:     registry,
		serviceByTag: make(map[string]adapter.Service),
	}
}

func (m *Manager) Start(stage adapter.StartStage) error {
	m.access.Lock()
	if m.started && m.stage >= stage {
		panic("already started")
	}
	m.started = true
	m.stage = stage
	services := m.services
	m.access.Unlock()
	for _, service := range services {
		name := "service/" + service.Type() + "[" + service.Tag() + "]"
		m.logger.Trace(stage, " ", name)
		startTime := time.Now()
		err := adapter.LegacyStart(service, stage)
		if err != nil {
			return E.Cause(err, stage, " ", name)
		}
		m.logger.Trace(stage, " ", name, " completed (", F.Seconds(time.Since(startTime).Seconds()), "s)")
	}
	return nil
}

func (m *Manager) Close() error {
	m.access.Lock()
	defer m.access.Unlock()
	if !m.started {
		return nil
	}
	m.started = false
	services := m.services
	m.services = nil
	monitor := taskmonitor.New(m.logger, C.StopTimeout)
	var err error
	for _, service := range services {
		name := "service/" + service.Type() + "[" + service.Tag() + "]"
		m.logger.Trace("close ", name)
		startTime := time.Now()
		monitor.Start("close ", name)
		err = E.Append(err, service.Close(), func(err error) error {
			return E.Cause(err, "close ", name)
		})
		monitor.Finish()
		m.logger.Trace("close ", name, " completed (", F.Seconds(time.Since(startTime).Seconds()), "s)")
	}
	return nil
}

func (m *Manager) Services() []adapter.Service {
	m.access.Lock()
	defer m.access.Unlock()
	return m.services
}

func (m *Manager) Get(tag string) (adapter.Service, bool) {
	m.access.Lock()
	service, found := m.serviceByTag[tag]
	m.access.Unlock()
	return service, found
}

func (m *Manager) Remove(tag string) error {
	m.access.Lock()
	service, found := m.serviceByTag[tag]
	if !found {
		m.access.Unlock()
		return os.ErrInvalid
	}
	delete(m.serviceByTag, tag)
	index := common.Index(m.services, func(it adapter.Service) bool {
		return it == service
	})
	if index == -1 {
		panic("invalid service index")
	}
	m.services = append(m.services[:index], m.services[index+1:]...)
	started := m.started
	m.access.Unlock()
	if started {
		return service.Close()
	}
	return nil
}

func (m *Manager) Create(ctx context.Context, logger log.ContextLogger, tag string, serviceType string, options any) error {
	service, err := m.registry.Create(ctx, logger, tag, serviceType, options)
	if err != nil {
		return err
	}
	m.access.Lock()
	defer m.access.Unlock()
	if m.started {
		name := "service/" + service.Type() + "[" + service.Tag() + "]"
		for _, stage := range adapter.ListStartStages {
			m.logger.Trace(stage, " ", name)
			startTime := time.Now()
			err = adapter.LegacyStart(service, stage)
			if err != nil {
				return E.Cause(err, stage, " ", name)
			}
			m.logger.Trace(stage, " ", name, " completed (", F.Seconds(time.Since(startTime).Seconds()), "s)")
		}
	}
	if existsService, loaded := m.serviceByTag[tag]; loaded {
		if m.started {
			err = existsService.Close()
			if err != nil {
				return E.Cause(err, "close service/", existsService.Type(), "[", existsService.Tag(), "]")
			}
		}
		existsIndex := common.Index(m.services, func(it adapter.Service) bool {
			return it == existsService
		})
		if existsIndex == -1 {
			panic("invalid service index")
		}
		m.services = append(m.services[:existsIndex], m.services[existsIndex+1:]...)
	}
	m.services = append(m.services, service)
	m.serviceByTag[tag] = service
	return nil
}
