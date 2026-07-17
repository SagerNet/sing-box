package v2rayxhttp

import (
	"net/http"
	"sync"
	"time"

	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

// xmuxConfig mirrors Xray's XHTTP client-reuse budgets. A zero range means
// unlimited (or disabled for the corresponding limit).
type xmuxConfig struct {
	maxConcurrency, maxConnections     byteRange
	maxReuse, maxRequests, maxReusable byteRange
	keepAlivePeriod                    time.Duration
}

func newXMuxConfig(options option.V2RayXHTTPXmuxOptions) (xmuxConfig, error) {
	if options.MaxConcurrency.To != 0 && options.MaxConnections.To != 0 {
		return xmuxConfig{}, E.New("xhttp xmux max_concurrency and max_connections are mutually exclusive")
	}
	config := xmuxConfig{
		maxConcurrency: optionalRange(options.MaxConcurrency),
		maxConnections: optionalRange(options.MaxConnections),
		maxReuse:       optionalRange(options.CMaxReuseTimes),
		maxRequests:    optionalRange(options.HMaxRequestTimes),
		maxReusable:    optionalRange(options.HMaxReusableSecs),
	}
	for _, value := range []byteRange{config.maxConcurrency, config.maxConnections, config.maxReuse, config.maxRequests, config.maxReusable} {
		if value.to != 0 && (!value.valid() || value.from < 0) {
			return xmuxConfig{}, E.New("invalid xhttp xmux range")
		}
	}
	if options.HKeepAlivePeriod < 0 {
		return xmuxConfig{}, E.New("invalid xhttp xmux h_keep_alive_period")
	}
	config.keepAlivePeriod = time.Duration(options.HKeepAlivePeriod) * time.Second
	return config, nil
}

func optionalRange(value option.V2RayXHTTPRange) byteRange {
	if value.To == 0 {
		return byteRange{}
	}
	return byteRange{from: value.From, to: value.To}
}

type xmuxClient struct {
	transport http.RoundTripper
	running   int
	// remainingReuse counts future logical-connection assignments. -1 means
	// unlimited. remainingRequests uses the same convention for HTTP requests.
	remainingReuse, remainingRequests int
	unusableAt                        time.Time
	retired                           bool
}

type xmuxManager struct {
	access                         sync.Mutex
	config                         xmuxConfig
	maxConcurrency, maxConnections int
	newTransport                   func() http.RoundTripper
	clients                        []*xmuxClient
	closed                         bool
}

func newXMuxManager(config xmuxConfig, newTransport func() http.RoundTripper) *xmuxManager {
	return &xmuxManager{
		config:         config,
		maxConcurrency: config.maxConcurrency.random(),
		maxConnections: config.maxConnections.random(),
		newTransport:   newTransport,
	}
}

func (m *xmuxManager) acquireConnection() *xmuxClient {
	m.access.Lock()
	defer m.access.Unlock()
	client := m.pickLocked(true)
	client.running++
	m.consumeRequestLocked(client)
	return client
}

// acquireRequest may select a different HTTP client once the logical
// connection's initial client has spent its request budget (packet-up).
func (m *xmuxManager) acquireRequest() *xmuxClient {
	m.access.Lock()
	defer m.access.Unlock()
	client := m.pickLocked(false)
	m.consumeRequestLocked(client)
	return client
}

func (m *xmuxManager) consumeRequest(client *xmuxClient) {
	m.access.Lock()
	defer m.access.Unlock()
	m.consumeRequestLocked(client)
}

// consumePacketRequest keeps packet-up uploads on their current client until
// its HTTP-request or time budget is spent, matching Xray's dynamic uploader.
func (m *xmuxManager) consumePacketRequest(client *xmuxClient) bool {
	m.access.Lock()
	defer m.access.Unlock()
	now := time.Now()
	if client.retired || client.remainingRequests == 0 || (!client.unusableAt.IsZero() && !now.Before(client.unusableAt)) {
		client.retired = true
		if client.running == 0 {
			closeIdleConnections(client.transport)
		}
		return false
	}
	m.consumeRequestLocked(client)
	return true
}

func (m *xmuxManager) consumeRequestLocked(client *xmuxClient) {
	if client.remainingRequests > 0 {
		client.remainingRequests--
	}
}

func (m *xmuxManager) doneConnection(client *xmuxClient) {
	m.access.Lock()
	if client.running > 0 {
		client.running--
	}
	if client.retired && client.running == 0 {
		closeIdleConnections(client.transport)
	}
	m.access.Unlock()
}

func (m *xmuxManager) pickLocked(connection bool) *xmuxClient {
	now := time.Now()
	for _, client := range m.clients {
		if !client.retired && (client.remainingReuse == 0 || client.remainingRequests == 0 || (!client.unusableAt.IsZero() && !now.Before(client.unusableAt))) {
			client.retired = true
			if client.running == 0 {
				closeIdleConnections(client.transport)
			}
		}
	}
	active := make([]*xmuxClient, 0, len(m.clients))
	for _, client := range m.clients {
		if !client.retired && (!connection || m.maxConcurrency == 0 || client.running < m.maxConcurrency) {
			active = append(active, client)
		}
	}
	activeCount := 0
	for _, client := range m.clients {
		if !client.retired {
			activeCount++
		}
	}
	if len(active) == 0 || (m.maxConnections > 0 && activeCount < m.maxConnections) {
		return m.newClientLocked()
	}
	client := active[byteRange{from: 0, to: int32(len(active) - 1)}.random()]
	if connection && client.remainingReuse > 0 {
		client.remainingReuse--
	}
	return client
}

func (m *xmuxManager) newClientLocked() *xmuxClient {
	client := &xmuxClient{transport: m.newTransport(), remainingReuse: -1, remainingRequests: -1}
	if value := m.config.maxReuse.random(); value > 0 {
		client.remainingReuse = value - 1
	}
	if value := m.config.maxRequests.random(); value > 0 {
		client.remainingRequests = value
	}
	if value := m.config.maxReusable.random(); value > 0 {
		client.unusableAt = time.Now().Add(time.Duration(value) * time.Second)
	}
	m.clients = append(m.clients, client)
	return client
}

func (m *xmuxManager) Close() {
	m.access.Lock()
	if m.closed {
		m.access.Unlock()
		return
	}
	m.closed = true
	for _, client := range m.clients {
		client.retired = true
		closeIdleConnections(client.transport)
	}
	m.access.Unlock()
}

func closeIdleConnections(transport http.RoundTripper) {
	if pool, ok := transport.(interface{ CloseIdleConnections() }); ok {
		pool.CloseIdleConnections()
	}
}
