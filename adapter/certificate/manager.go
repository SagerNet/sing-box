package certificate

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

var _ adapter.CertificateProviderManager = (*Manager)(nil)

type Manager struct {
	logger        log.ContextLogger
	registry      adapter.CertificateProviderRegistry
	access        sync.Mutex
	started       bool
	stage         adapter.StartStage
	providers     []adapter.CertificateProviderService
	providerByTag map[string]adapter.CertificateProviderService
}

func NewManager(logger log.ContextLogger, registry adapter.CertificateProviderRegistry) *Manager {
	return &Manager{
		logger:        logger,
		registry:      registry,
		providerByTag: make(map[string]adapter.CertificateProviderService),
	}
}

func (m *Manager) Start(stage adapter.StartStage) error {
	m.access.Lock()
	if m.started && m.stage >= stage {
		panic("already started")
	}
	m.started = true
	m.stage = stage
	providers := m.providers
	m.access.Unlock()
	for _, provider := range providers {
		name := "certificate-provider/" + provider.Type() + "[" + provider.Tag() + "]"
		m.logger.Trace(stage, " ", name)
		startTime := time.Now()
		err := adapter.LegacyStart(provider, stage)
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
	providers := m.providers
	m.providers = nil
	monitor := taskmonitor.New(m.logger, C.StopTimeout)
	var err error
	for _, provider := range providers {
		name := "certificate-provider/" + provider.Type() + "[" + provider.Tag() + "]"
		m.logger.Trace("close ", name)
		startTime := time.Now()
		monitor.Start("close ", name)
		err = E.Append(err, provider.Close(), func(err error) error {
			return E.Cause(err, "close ", name)
		})
		monitor.Finish()
		m.logger.Trace("close ", name, " completed (", F.Seconds(time.Since(startTime).Seconds()), "s)")
	}
	return err
}

func (m *Manager) CertificateProviders() []adapter.CertificateProviderService {
	m.access.Lock()
	defer m.access.Unlock()
	return m.providers
}

func (m *Manager) Get(tag string) (adapter.CertificateProviderService, bool) {
	m.access.Lock()
	provider, found := m.providerByTag[tag]
	m.access.Unlock()
	return provider, found
}

func (m *Manager) Remove(tag string) error {
	m.access.Lock()
	provider, found := m.providerByTag[tag]
	if !found {
		m.access.Unlock()
		return os.ErrInvalid
	}
	delete(m.providerByTag, tag)
	index := common.Index(m.providers, func(it adapter.CertificateProviderService) bool {
		return it == provider
	})
	if index == -1 {
		panic("invalid certificate provider index")
	}
	m.providers = append(m.providers[:index], m.providers[index+1:]...)
	started := m.started
	m.access.Unlock()
	if started {
		return provider.Close()
	}
	return nil
}

func (m *Manager) Create(ctx context.Context, logger log.ContextLogger, tag string, providerType string, options any) error {
	provider, err := m.registry.Create(ctx, logger, tag, providerType, options)
	if err != nil {
		return err
	}
	m.access.Lock()
	defer m.access.Unlock()
	if m.started {
		name := "certificate-provider/" + provider.Type() + "[" + provider.Tag() + "]"
		for _, stage := range adapter.ListStartStages {
			m.logger.Trace(stage, " ", name)
			startTime := time.Now()
			err = adapter.LegacyStart(provider, stage)
			if err != nil {
				return E.Cause(err, stage, " ", name)
			}
			m.logger.Trace(stage, " ", name, " completed (", F.Seconds(time.Since(startTime).Seconds()), "s)")
		}
	}
	if existsProvider, loaded := m.providerByTag[tag]; loaded {
		if m.started {
			err = existsProvider.Close()
			if err != nil {
				return E.Cause(err, "close certificate-provider/", existsProvider.Type(), "[", existsProvider.Tag(), "]")
			}
		}
		existsIndex := common.Index(m.providers, func(it adapter.CertificateProviderService) bool {
			return it == existsProvider
		})
		if existsIndex == -1 {
			panic("invalid certificate provider index")
		}
		m.providers = append(m.providers[:existsIndex], m.providers[existsIndex+1:]...)
	}
	m.providers = append(m.providers, provider)
	m.providerByTag[tag] = provider
	return nil
}
