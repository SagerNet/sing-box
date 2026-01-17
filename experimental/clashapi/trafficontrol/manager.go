package trafficontrol

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/common/compatible"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/observable"
	"github.com/sagernet/sing/common/x/list"

	"github.com/gofrs/uuid/v5"
)

type ConnectionEventType int

const (
	ConnectionEventNew ConnectionEventType = iota
	ConnectionEventUpdate
	ConnectionEventClosed
)

type ConnectionEvent struct {
	Type          ConnectionEventType
	ID            uuid.UUID
	Metadata      *TrackerMetadata
	UplinkDelta   int64
	DownlinkDelta int64
	ClosedAt      time.Time
}

const closedConnectionsLimit = 1000

type Manager struct {
	uploadTotal   atomic.Int64
	downloadTotal atomic.Int64

	connections             compatible.Map[uuid.UUID, Tracker]
	closedConnectionsAccess sync.Mutex
	closedConnections       list.List[TrackerMetadata]
	memory                  uint64

	eventSubscriber *observable.Subscriber[ConnectionEvent]
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) SetEventHook(subscriber *observable.Subscriber[ConnectionEvent]) {
	m.eventSubscriber = subscriber
}

func (m *Manager) Join(c Tracker) {
	metadata := c.Metadata()
	m.connections.Store(metadata.ID, c)
	if m.eventSubscriber != nil {
		m.eventSubscriber.Emit(ConnectionEvent{
			Type:     ConnectionEventNew,
			ID:       metadata.ID,
			Metadata: metadata,
		})
	}
}

func (m *Manager) Leave(c Tracker) {
	metadata := c.Metadata()
	_, loaded := m.connections.LoadAndDelete(metadata.ID)
	if loaded {
		closedAt := time.Now()
		metadata.ClosedAt = closedAt
		metadataCopy := *metadata
		m.closedConnectionsAccess.Lock()
		if m.closedConnections.Len() >= closedConnectionsLimit {
			m.closedConnections.PopFront()
		}
		m.closedConnections.PushBack(metadataCopy)
		m.closedConnectionsAccess.Unlock()
		if m.eventSubscriber != nil {
			m.eventSubscriber.Emit(ConnectionEvent{
				Type:     ConnectionEventClosed,
				ID:       metadata.ID,
				Metadata: &metadataCopy,
				ClosedAt: closedAt,
			})
		}
	}
}

func (m *Manager) PushUploaded(size int64) {
	m.uploadTotal.Add(size)
}

func (m *Manager) PushDownloaded(size int64) {
	m.downloadTotal.Add(size)
}

func (m *Manager) Total() (up int64, down int64) {
	return m.uploadTotal.Load(), m.downloadTotal.Load()
}

func (m *Manager) ConnectionsLen() int {
	return m.connections.Len()
}

func (m *Manager) Connections() []*TrackerMetadata {
	var connections []*TrackerMetadata
	m.connections.Range(func(_ uuid.UUID, value Tracker) bool {
		connections = append(connections, value.Metadata())
		return true
	})
	return connections
}

func (m *Manager) ClosedConnections() []*TrackerMetadata {
	m.closedConnectionsAccess.Lock()
	values := m.closedConnections.Array()
	m.closedConnectionsAccess.Unlock()
	if len(values) == 0 {
		return nil
	}
	connections := make([]*TrackerMetadata, len(values))
	for i := range values {
		connections[i] = &values[i]
	}
	return connections
}

func (m *Manager) Connection(id uuid.UUID) Tracker {
	connection, loaded := m.connections.Load(id)
	if !loaded {
		return nil
	}
	return connection
}

func (m *Manager) Snapshot() *Snapshot {
	var connections []Tracker
	m.connections.Range(func(_ uuid.UUID, value Tracker) bool {
		if value.Metadata().OutboundType != C.TypeDNS {
			connections = append(connections, value)
		}
		return true
	})

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	m.memory = memStats.StackInuse + memStats.HeapInuse + memStats.HeapIdle - memStats.HeapReleased

	return &Snapshot{
		Upload:      m.uploadTotal.Load(),
		Download:    m.downloadTotal.Load(),
		Connections: connections,
		Memory:      m.memory,
	}
}

func (m *Manager) ResetStatistic() {
	m.uploadTotal.Store(0)
	m.downloadTotal.Store(0)
}

type Snapshot struct {
	Download    int64
	Upload      int64
	Connections []Tracker
	Memory      uint64
}

func (s *Snapshot) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"downloadTotal": s.Download,
		"uploadTotal":   s.Upload,
		"connections":   common.Map(s.Connections, func(t Tracker) *TrackerMetadata { return t.Metadata() }),
		"memory":        s.Memory,
	})
}
