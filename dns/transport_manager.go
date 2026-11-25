package dns

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ adapter.DNSTransportManager = (*TransportManager)(nil)

type TransportManager struct {
	registry         adapter.DNSTransportRegistry
	outbound         adapter.OutboundManager
	defaultTag       string
	access           sync.RWMutex
	transports       []adapter.DNSTransport
	transportByTag   map[string]adapter.DNSTransport
	defaultTransport adapter.DNSTransport
	fakeIPTransport  adapter.FakeIPTransport
	sortedTransports []adapter.DNSTransport
}

func NewTransportManager(registry adapter.DNSTransportRegistry, outbound adapter.OutboundManager, defaultTag string) *TransportManager {
	return &TransportManager{
		registry:       registry,
		outbound:       outbound,
		defaultTag:     defaultTag,
		transportByTag: make(map[string]adapter.DNSTransport),
	}
}

func (m *TransportManager) Initialize(defaultTransportFallback func() (adapter.DNSTransport, error)) error {
	m.access.Lock()
	defer m.access.Unlock()
	if m.defaultTag != "" && m.defaultTransport == nil {
		return E.New("default DNS server not found: ", m.defaultTag)
	}
	if m.defaultTransport == nil {
		defaultTransport, err := defaultTransportFallback()
		if err != nil {
			return E.Cause(err, "default DNS server fallback")
		}
		m.transports = append(m.transports, defaultTransport)
		m.transportByTag[defaultTransport.Tag()] = defaultTransport
		m.defaultTransport = defaultTransport
	}
	sortedTransports, err := m.sortTransports(m.transports)
	if err != nil {
		return err
	}
	m.sortedTransports = sortedTransports
	return nil
}

func (m *TransportManager) Start(stage adapter.StartStage, scope *adapter.Scope) error {
	m.access.RLock()
	sortedTransports := m.sortedTransports
	m.access.RUnlock()
	for _, transport := range sortedTransports {
		name := "dns/" + transport.Type() + "[" + transport.Tag() + "]"
		err := scope.Start(name, transport, stage)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *TransportManager) sortTransports(transports []adapter.DNSTransport) ([]adapter.DNSTransport, error) {
	sortedTransports := make([]adapter.DNSTransport, 0, len(transports))
	sorted := make(map[string]bool)
	for {
		canContinue := false
	sortOne:
		for _, transportToSort := range transports {
			transportTag := transportToSort.Tag()
			if sorted[transportTag] {
				continue
			}
			dependencies := transportToSort.Dependencies()
			for _, dependency := range dependencies {
				if !sorted[dependency] {
					continue sortOne
				}
			}
			sorted[transportTag] = true
			canContinue = true
			sortedTransports = append(sortedTransports, transportToSort)
		}
		if len(sorted) == len(transports) {
			break
		}
		if canContinue {
			continue
		}
		currentTransport := common.Find(transports, func(it adapter.DNSTransport) bool {
			return !sorted[it.Tag()]
		})
		var lintTransport func(oTree []string, oCurrent adapter.DNSTransport) error
		lintTransport = func(oTree []string, oCurrent adapter.DNSTransport) error {
			problemTransportTag := common.Find(oCurrent.Dependencies(), func(it string) bool {
				return !sorted[it]
			})
			if common.Contains(oTree, problemTransportTag) {
				return E.New("circular server dependency: ", strings.Join(oTree, " -> "), " -> ", problemTransportTag)
			}
			problemTransport := m.transportByTag[problemTransportTag]
			if problemTransport == nil {
				return E.New("dependency[", problemTransportTag, "] not found for server[", oCurrent.Tag(), "]")
			}
			return lintTransport(append(oTree, problemTransportTag), problemTransport)
		}
		return nil, lintTransport([]string{currentTransport.Tag()}, currentTransport)
	}
	return sortedTransports, nil
}

func (m *TransportManager) Transports() []adapter.DNSTransport {
	m.access.RLock()
	defer m.access.RUnlock()
	return m.transports
}

func (m *TransportManager) Transport(tag string) (adapter.DNSTransport, bool) {
	m.access.RLock()
	outbound, found := m.transportByTag[tag]
	m.access.RUnlock()
	return outbound, found
}

func (m *TransportManager) Default() adapter.DNSTransport {
	m.access.RLock()
	defer m.access.RUnlock()
	return m.defaultTransport
}

func (m *TransportManager) FakeIP() adapter.FakeIPTransport {
	m.access.RLock()
	defer m.access.RUnlock()
	return m.fakeIPTransport
}

func (m *TransportManager) Create(ctx context.Context, logger log.ContextLogger, tag string, transportType string, options any) error {
	if tag == "" {
		return os.ErrInvalid
	}
	transport, err := m.registry.CreateDNSTransport(ctx, logger, tag, transportType, options)
	if err != nil {
		return err
	}
	m.access.Lock()
	defer m.access.Unlock()
	_, loaded := m.transportByTag[tag]
	if loaded {
		return E.New("duplicate DNS server tag: ", tag)
	}
	m.transports = append(m.transports, transport)
	m.transportByTag[tag] = transport
	if tag == m.defaultTag || (m.defaultTag == "" && m.defaultTransport == nil) {
		if transport.Type() == C.DNSTypeFakeIP {
			return E.New("default server cannot be fakeip")
		}
		m.defaultTransport = transport
	}
	if transport.Type() == C.DNSTypeFakeIP {
		if m.fakeIPTransport != nil {
			return E.New("multiple fakeip server are not supported")
		}
		m.fakeIPTransport = transport.(adapter.FakeIPTransport)
	}
	return nil
}
