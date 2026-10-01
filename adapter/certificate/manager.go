package certificate

import (
	"context"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ adapter.CertificateProviderManager = (*Manager)(nil)

type Manager struct {
	registry      adapter.CertificateProviderRegistry
	access        sync.Mutex
	providers     []adapter.CertificateProviderService
	providerByTag map[string]adapter.CertificateProviderService
}

func NewManager(registry adapter.CertificateProviderRegistry) *Manager {
	return &Manager{
		registry:      registry,
		providerByTag: make(map[string]adapter.CertificateProviderService),
	}
}

func (m *Manager) Start(stage adapter.StartStage, scope *adapter.Scope) error {
	m.access.Lock()
	providers := m.providers
	m.access.Unlock()
	for _, provider := range providers {
		name := "certificate-provider/" + provider.Type() + "[" + provider.Tag() + "]"
		err := scope.Start(name, provider, stage)
		if err != nil {
			return err
		}
	}
	return nil
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

func (m *Manager) Create(ctx context.Context, logger log.ContextLogger, tag string, providerType string, options any) error {
	provider, err := m.registry.Create(ctx, logger, tag, providerType, options)
	if err != nil {
		return err
	}
	m.access.Lock()
	defer m.access.Unlock()
	_, loaded := m.providerByTag[tag]
	if loaded {
		return E.New("duplicate certificate provider tag: ", tag)
	}
	m.providers = append(m.providers, provider)
	m.providerByTag[tag] = provider
	return nil
}
