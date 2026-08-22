package trafficcontrol

import (
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/compatible"
	"github.com/sagernet/sing/common/cleanup"
	"github.com/sagernet/sing/common/observable"
	"github.com/sagernet/sing/common/x/list"

	"github.com/gofrs/uuid/v5"
)

type ConnectionEventType int

const (
	ConnectionEventNew ConnectionEventType = iota
	ConnectionEventClosed
)

type ConnectionEvent struct {
	Type     ConnectionEventType
	ID       uuid.UUID
	Metadata *TrackerMetadata
	ClosedAt time.Time
}

const closedConnectionsLimit = 1000

var (
	_ adapter.ConnectionTracker = (*Manager)(nil)
	_ adapter.LifecycleService  = (*Manager)(nil)
)

type Manager struct {
	outbound adapter.OutboundManager

	connections             compatible.Map[uuid.UUID, Tracker]
	closedConnectionsAccess sync.Mutex
	closedConnections       list.List[TrackerMetadata]
	closedUploadTotal       int64
	closedDownloadTotal     int64

	eventSubscriber *observable.Subscriber[ConnectionEvent]
	eventObserver   *observable.Observer[ConnectionEvent]
	cleaner         *cleanup.Cleaner
}

func NewManager(outbound adapter.OutboundManager) *Manager {
	return &Manager{
		outbound:        outbound,
		eventSubscriber: observable.NewSubscriber[ConnectionEvent](256),
	}
}

func (m *Manager) Name() string {
	return "traffic manager"
}

func (m *Manager) Start(stage adapter.StartStage) error {
	if stage == adapter.StartStateInitialize {
		m.eventObserver = observable.NewObserver(m.eventSubscriber, 64)
		m.cleaner = cleanup.Add(m.Clear)
	}
	return nil
}

func (m *Manager) Close() error {
	if m.cleaner != nil {
		m.cleaner.Close()
	}
	if m.eventObserver != nil {
		return m.eventObserver.Close()
	}
	return nil
}

func (m *Manager) SubscribeEvents() (observable.Subscription[ConnectionEvent], <-chan struct{}, error) {
	return m.eventObserver.Subscribe()
}

func (m *Manager) UnSubscribeEvents(subscription observable.Subscription[ConnectionEvent]) {
	m.eventObserver.UnSubscribe(subscription)
}

func (m *Manager) join(tracker Tracker) {
	metadata := tracker.Metadata()
	m.connections.Store(metadata.ID, tracker)
	m.eventSubscriber.Emit(ConnectionEvent{
		Type:     ConnectionEventNew,
		ID:       metadata.ID,
		Metadata: metadata,
	})
}

func (m *Manager) leave(tracker Tracker) {
	metadata := tracker.Metadata()
	closedAt := time.Now()
	m.closedConnectionsAccess.Lock()
	_, loaded := m.connections.LoadAndDelete(metadata.ID)
	if !loaded {
		m.closedConnectionsAccess.Unlock()
		return
	}
	metadata.ClosedAt = closedAt
	metadataCopy := *metadata
	if m.closedConnections.Len() >= closedConnectionsLimit {
		evicted := m.closedConnections.PopFront()
		m.closedUploadTotal += evicted.Upload.Load()
		m.closedDownloadTotal += evicted.Download.Load()
	}
	m.closedConnections.PushBack(metadataCopy)
	m.closedConnectionsAccess.Unlock()
	m.eventSubscriber.Emit(ConnectionEvent{
		Type:     ConnectionEventClosed,
		ID:       metadata.ID,
		Metadata: &metadataCopy,
		ClosedAt: closedAt,
	})
}

func (m *Manager) Total() (uplinkTotal int64, downlinkTotal int64) {
	m.closedConnectionsAccess.Lock()
	defer m.closedConnectionsAccess.Unlock()
	uplinkTotal = m.closedUploadTotal
	downlinkTotal = m.closedDownloadTotal
	for element := m.closedConnections.Front(); element != nil; element = element.Next() {
		uplinkTotal += element.Value.Upload.Load()
		downlinkTotal += element.Value.Download.Load()
	}
	m.connections.Range(func(_ uuid.UUID, tracker Tracker) bool {
		metadata := tracker.Metadata()
		uplinkTotal += metadata.Upload.Load()
		downlinkTotal += metadata.Download.Load()
		return true
	})
	return
}

func (m *Manager) ConnectionsLen() int {
	return m.connections.Len()
}

func (m *Manager) Connections() []*TrackerMetadata {
	var connections []*TrackerMetadata
	m.connections.Range(func(_ uuid.UUID, tracker Tracker) bool {
		connections = append(connections, tracker.Metadata())
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

func (m *Manager) CloseAllConnections() {
	m.connections.Range(func(_ uuid.UUID, tracker Tracker) bool {
		tracker.Close()
		return true
	})
}

func (m *Manager) Clear() {
	m.closedConnectionsAccess.Lock()
	defer m.closedConnectionsAccess.Unlock()
	for element := m.closedConnections.Front(); element != nil; element = element.Next() {
		m.closedUploadTotal += element.Value.Upload.Load()
		m.closedDownloadTotal += element.Value.Download.Load()
	}
	m.closedConnections.Init()
}
