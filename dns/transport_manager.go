package dns

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
)

var _ adapter.DNSTransportManager = (*TransportManager)(nil)

type TransportManager struct {
	logger                   log.ContextLogger
	registry                 adapter.DNSTransportRegistry
	outbound                 adapter.OutboundManager
	defaultTag               string
	access                   sync.RWMutex
	started                  bool
	stage                    adapter.StartStage
	transports               []adapter.DNSTransport
	transportByTag           map[string]adapter.DNSTransport
	dependByTag              map[string][]string
	defaultTransport         adapter.DNSTransport
	defaultTransportFallback func() (adapter.DNSTransport, error)
	fakeIPTransport          adapter.FakeIPTransport
}

func NewTransportManager(logger logger.ContextLogger, registry adapter.DNSTransportRegistry, outbound adapter.OutboundManager, defaultTag string) *TransportManager {
	return &TransportManager{
		logger:         logger,
		registry:       registry,
		outbound:       outbound,
		defaultTag:     defaultTag,
		transportByTag: make(map[string]adapter.DNSTransport),
		dependByTag:    make(map[string][]string),
	}
}

func (m *TransportManager) Initialize(defaultTransportFallback func() (adapter.DNSTransport, error)) {
	m.defaultTransportFallback = defaultTransportFallback
}

func (m *TransportManager) Start(stage adapter.StartStage) error {
	m.access.Lock()
	if m.started && m.stage >= stage {
		panic("already started")
	}
	m.started = true
	m.stage = stage
	if stage == adapter.StartStateStart {
		if m.defaultTag != "" && m.defaultTransport == nil {
			m.access.Unlock()
			return E.New("default DNS server not found: ", m.defaultTag)
		}
		if m.defaultTransport == nil {
			defaultTransport, err := m.defaultTransportFallback()
			if err != nil {
				m.access.Unlock()
				return E.Cause(err, "default DNS server fallback")
			}
			m.transports = append(m.transports, defaultTransport)
			m.transportByTag[defaultTransport.Tag()] = defaultTransport
			m.defaultTransport = defaultTransport
		}
		transports := m.transports
		m.access.Unlock()
		return m.startTransports(transports)
	} else {
		transports := m.transports
		m.access.Unlock()
		for _, outbound := range transports {
			err := adapter.LegacyStart(outbound, stage)
			if err != nil {
				return E.Cause(err, stage, " dns/", outbound.Type(), "[", outbound.Tag(), "]")
			}
		}
	}
	return nil
}

func (m *TransportManager) startTransports(transports []adapter.DNSTransport) error {
	monitor := taskmonitor.New(m.logger, C.StartTimeout)
	started := make(map[string]bool)
	for {
		canContinue := false
	startOne:
		for _, transportToStart := range transports {
			transportTag := transportToStart.Tag()
			if started[transportTag] {
				continue
			}
			dependencies := transportToStart.Dependencies()
			for _, dependency := range dependencies {
				if !started[dependency] {
					continue startOne
				}
			}
			started[transportTag] = true
			canContinue = true
			if starter, isStarter := transportToStart.(adapter.Lifecycle); isStarter {
				monitor.Start("start dns/", transportToStart.Type(), "[", transportTag, "]")
				err := starter.Start(adapter.StartStateStart)
				monitor.Finish()
				if err != nil {
					return E.Cause(err, "start dns/", transportToStart.Type(), "[", transportTag, "]")
				}
			}
		}
		if len(started) == len(transports) {
			break
		}
		if canContinue {
			continue
		}
		currentTransport := common.Find(transports, func(it adapter.DNSTransport) bool {
			return !started[it.Tag()]
		})
		var lintTransport func(oTree []string, oCurrent adapter.DNSTransport) error
		lintTransport = func(oTree []string, oCurrent adapter.DNSTransport) error {
			problemTransportTag := common.Find(oCurrent.Dependencies(), func(it string) bool {
				return !started[it]
			})
			if common.Contains(oTree, problemTransportTag) {
				return E.New("circular server dependency: ", strings.Join(oTree, " -> "), " -> ", problemTransportTag)
			}
			m.access.Lock()
			problemTransport := m.transportByTag[problemTransportTag]
			m.access.Unlock()
			if problemTransport == nil {
				return E.New("dependency[", problemTransportTag, "] not found for server[", oCurrent.Tag(), "]")
			}
			return lintTransport(append(oTree, problemTransportTag), problemTransport)
		}
		return lintTransport([]string{currentTransport.Tag()}, currentTransport)
	}
	return nil
}

func (m *TransportManager) Close() error {
	monitor := taskmonitor.New(m.logger, C.StopTimeout)
	m.access.Lock()
	if !m.started {
		m.access.Unlock()
		return nil
	}
	m.started = false
	transports := m.transports
	m.transports = nil
	m.access.Unlock()
	var err error
	for _, transport := range transports {
		if closer, isCloser := transport.(io.Closer); isCloser {
			monitor.Start("close server/", transport.Type(), "[", transport.Tag(), "]")
			err = E.Append(err, closer.Close(), func(err error) error {
				return E.Cause(err, "close server/", transport.Type(), "[", transport.Tag(), "]")
			})
			monitor.Finish()
		}
	}
	return nil
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

func (m *TransportManager) Remove(tag string) error {
	m.access.Lock()
	defer m.access.Unlock()
	transport, found := m.transportByTag[tag]
	if !found {
		return os.ErrInvalid
	}
	delete(m.transportByTag, tag)
	index := common.Index(m.transports, func(it adapter.DNSTransport) bool {
		return it == transport
	})
	if index == -1 {
		panic("invalid inbound index")
	}
	m.transports = append(m.transports[:index], m.transports[index+1:]...)
	started := m.started
	if m.defaultTransport == transport {
		if len(m.transports) > 0 {
			nextTransport := m.transports[0]
			if nextTransport.Type() != C.DNSTypeFakeIP {
				return E.New("default server cannot be fakeip")
			}
			m.defaultTransport = nextTransport
			m.logger.Info("updated default server to ", m.defaultTransport.Tag())
		} else {
			m.defaultTransport = nil
		}
	}
	dependBy := m.dependByTag[tag]
	if len(dependBy) > 0 {
		return E.New("server[", tag, "] is depended by ", strings.Join(dependBy, ", "))
	}
	dependencies := transport.Dependencies()
	for _, dependency := range dependencies {
		if len(m.dependByTag[dependency]) == 1 {
			delete(m.dependByTag, dependency)
		} else {
			m.dependByTag[dependency] = common.Filter(m.dependByTag[dependency], func(it string) bool {
				return it != tag
			})
		}
	}
	if started {
		transport.Close()
	}
	return nil
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
	if m.started {
		for _, stage := range adapter.ListStartStages {
			err = adapter.LegacyStart(transport, stage)
			if err != nil {
				return E.Cause(err, stage, " dns/", transport.Type(), "[", transport.Tag(), "]")
			}
		}
	}
	if existsTransport, loaded := m.transportByTag[tag]; loaded {
		if m.started {
			err = common.Close(existsTransport)
			if err != nil {
				return E.Cause(err, "close dns/", existsTransport.Type(), "[", existsTransport.Tag(), "]")
			}
		}
		existsIndex := common.Index(m.transports, func(it adapter.DNSTransport) bool {
			return it == existsTransport
		})
		if existsIndex == -1 {
			panic("invalid inbound index")
		}
		m.transports = append(m.transports[:existsIndex], m.transports[existsIndex+1:]...)
	}
	m.transports = append(m.transports, transport)
	m.transportByTag[tag] = transport
	dependencies := transport.Dependencies()
	for _, dependency := range dependencies {
		m.dependByTag[dependency] = append(m.dependByTag[dependency], tag)
	}
	if tag == m.defaultTag || (m.defaultTag == "" && m.defaultTransport == nil) {
		if transport.Type() == C.DNSTypeFakeIP {
			return E.New("default server cannot be fakeip")
		}
		m.defaultTransport = transport
		if m.started {
			m.logger.Info("updated default server to ", transport.Tag())
		}
	}
	if transport.Type() == C.DNSTypeFakeIP {
		if m.fakeIPTransport != nil {
			return E.New("multiple fakeip server are not supported")
		}
		m.fakeIPTransport = transport.(adapter.FakeIPTransport)
	}
	return nil
}
