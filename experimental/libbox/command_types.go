package libbox

import (
	"slices"
	"strings"
	"time"

	"github.com/sagernet/sing-box/daemon"
	M "github.com/sagernet/sing/common/metadata"
)

type StatusMessage struct {
	Memory           int64
	Goroutines       int32
	ConnectionsIn    int32
	ConnectionsOut   int32
	TrafficAvailable bool
	Uplink           int64
	Downlink         int64
	UplinkTotal      int64
	DownlinkTotal    int64
}

type SystemProxyStatus struct {
	Available bool
	Enabled   bool
}

type OutboundGroup struct {
	Tag        string
	Type       string
	Selectable bool
	Selected   string
	IsExpand   bool
	ItemList   []*OutboundGroupItem
}

func (g *OutboundGroup) GetItems() OutboundGroupItemIterator {
	return newIterator(g.ItemList)
}

type OutboundGroupIterator interface {
	Next() *OutboundGroup
	HasNext() bool
}

type OutboundGroupItem struct {
	Tag          string
	Type         string
	URLTestTime  int64
	URLTestDelay int32
}

type OutboundGroupItemIterator interface {
	Next() *OutboundGroupItem
	HasNext() bool
}

const (
	ConnectionStateAll = iota
	ConnectionStateActive
	ConnectionStateClosed
)

const (
	ConnectionEventNew = iota
	ConnectionEventUpdate
	ConnectionEventClosed
)

const (
	closedConnectionMaxAge = int64((5 * time.Minute) / time.Millisecond)
)

type ConnectionEvent struct {
	Type          int32
	ID            string
	Connection    *Connection
	UplinkDelta   int64
	DownlinkDelta int64
	ClosedAt      int64
}

type ConnectionEvents struct {
	Reset  bool
	events []*ConnectionEvent
}

func (c *ConnectionEvents) Iterator() ConnectionEventIterator {
	return newIterator(c.events)
}

type ConnectionEventIterator interface {
	Next() *ConnectionEvent
	HasNext() bool
}

type Connections struct {
	connectionMap map[string]*Connection
	input         []Connection
	filtered      []Connection
	filterState   int32
	filterApplied bool
}

func NewConnections() *Connections {
	return &Connections{
		connectionMap: make(map[string]*Connection),
	}
}

func (c *Connections) ApplyEvents(events *ConnectionEvents) {
	if events == nil {
		return
	}
	if events.Reset {
		c.connectionMap = make(map[string]*Connection)
	}

	for _, event := range events.events {
		switch event.Type {
		case ConnectionEventNew:
			if event.Connection != nil {
				conn := *event.Connection
				c.connectionMap[event.ID] = &conn
			}
		case ConnectionEventUpdate:
			if conn, ok := c.connectionMap[event.ID]; ok {
				conn.Uplink = event.UplinkDelta
				conn.Downlink = event.DownlinkDelta
				conn.UplinkTotal += event.UplinkDelta
				conn.DownlinkTotal += event.DownlinkDelta
			}
		case ConnectionEventClosed:
			if event.Connection != nil {
				conn := *event.Connection
				conn.ClosedAt = event.ClosedAt
				conn.Uplink = 0
				conn.Downlink = 0
				c.connectionMap[event.ID] = &conn
				continue
			}
			if conn, ok := c.connectionMap[event.ID]; ok {
				conn.ClosedAt = event.ClosedAt
				conn.Uplink = 0
				conn.Downlink = 0
			}
		}
	}

	c.evictClosedConnections(time.Now().UnixMilli())
	c.input = c.input[:0]
	for _, conn := range c.connectionMap {
		c.input = append(c.input, *conn)
	}
	if c.filterApplied {
		c.FilterState(c.filterState)
	} else {
		c.filtered = c.filtered[:0]
		c.filtered = append(c.filtered, c.input...)
	}
}

func (c *Connections) evictClosedConnections(nowMilliseconds int64) {
	for id, conn := range c.connectionMap {
		if conn.ClosedAt == 0 {
			continue
		}
		if nowMilliseconds-conn.ClosedAt > closedConnectionMaxAge {
			delete(c.connectionMap, id)
		}
	}
}

func (c *Connections) FilterState(state int32) {
	c.filterApplied = true
	c.filterState = state
	c.filtered = c.filtered[:0]
	switch state {
	case ConnectionStateAll:
		c.filtered = append(c.filtered, c.input...)
	case ConnectionStateActive:
		for _, connection := range c.input {
			if connection.ClosedAt == 0 {
				c.filtered = append(c.filtered, connection)
			}
		}
	case ConnectionStateClosed:
		for _, connection := range c.input {
			if connection.ClosedAt != 0 {
				c.filtered = append(c.filtered, connection)
			}
		}
	}
}

func (c *Connections) SortByDate() {
	slices.SortStableFunc(c.filtered, func(x, y Connection) int {
		if x.CreatedAt < y.CreatedAt {
			return 1
		} else if x.CreatedAt > y.CreatedAt {
			return -1
		} else {
			return strings.Compare(y.ID, x.ID)
		}
	})
}

func (c *Connections) SortByTraffic() {
	slices.SortStableFunc(c.filtered, func(x, y Connection) int {
		xTraffic := x.Uplink + x.Downlink
		yTraffic := y.Uplink + y.Downlink
		if xTraffic < yTraffic {
			return 1
		} else if xTraffic > yTraffic {
			return -1
		} else {
			return strings.Compare(y.ID, x.ID)
		}
	})
}

func (c *Connections) SortByTrafficTotal() {
	slices.SortStableFunc(c.filtered, func(x, y Connection) int {
		xTraffic := x.UplinkTotal + x.DownlinkTotal
		yTraffic := y.UplinkTotal + y.DownlinkTotal
		if xTraffic < yTraffic {
			return 1
		} else if xTraffic > yTraffic {
			return -1
		} else {
			return strings.Compare(y.ID, x.ID)
		}
	})
}

func (c *Connections) Iterator() ConnectionIterator {
	return newPtrIterator(c.filtered)
}

type ProcessInfo struct {
	ProcessID   int64
	UserID      int32
	UserName    string
	ProcessPath string
	PackageName string
}

type Connection struct {
	ID            string
	Inbound       string
	InboundType   string
	IPVersion     int32
	Network       string
	Source        string
	Destination   string
	Domain        string
	Protocol      string
	User          string
	FromOutbound  string
	CreatedAt     int64
	ClosedAt      int64
	Uplink        int64
	Downlink      int64
	UplinkTotal   int64
	DownlinkTotal int64
	Rule          string
	Outbound      string
	OutboundType  string
	ChainList     []string
	ProcessInfo   *ProcessInfo
}

func (c *Connection) Chain() StringIterator {
	return newIterator(c.ChainList)
}

func (c *Connection) DisplayDestination() string {
	destination := M.ParseSocksaddr(c.Destination)
	if destination.IsIP() && c.Domain != "" {
		destination = M.Socksaddr{
			Fqdn: c.Domain,
			Port: destination.Port,
		}
		return destination.String()
	}
	return c.Destination
}

type ConnectionIterator interface {
	Next() *Connection
	HasNext() bool
}

func StatusMessageFromGRPC(status *daemon.Status) *StatusMessage {
	if status == nil {
		return nil
	}
	return &StatusMessage{
		Memory:           int64(status.Memory),
		Goroutines:       status.Goroutines,
		ConnectionsIn:    status.ConnectionsIn,
		ConnectionsOut:   status.ConnectionsOut,
		TrafficAvailable: status.TrafficAvailable,
		Uplink:           status.Uplink,
		Downlink:         status.Downlink,
		UplinkTotal:      status.UplinkTotal,
		DownlinkTotal:    status.DownlinkTotal,
	}
}

func OutboundGroupIteratorFromGRPC(groups *daemon.Groups) OutboundGroupIterator {
	if groups == nil || len(groups.Group) == 0 {
		return newIterator([]*OutboundGroup{})
	}
	var libboxGroups []*OutboundGroup
	for _, g := range groups.Group {
		libboxGroup := &OutboundGroup{
			Tag:        g.Tag,
			Type:       g.Type,
			Selectable: g.Selectable,
			Selected:   g.Selected,
			IsExpand:   g.IsExpand,
		}
		for _, item := range g.Items {
			libboxGroup.ItemList = append(libboxGroup.ItemList, &OutboundGroupItem{
				Tag:          item.Tag,
				Type:         item.Type,
				URLTestTime:  item.UrlTestTime,
				URLTestDelay: item.UrlTestDelay,
			})
		}
		libboxGroups = append(libboxGroups, libboxGroup)
	}
	return newIterator(libboxGroups)
}

func ConnectionFromGRPC(conn *daemon.Connection) Connection {
	var processInfo *ProcessInfo
	if conn.ProcessInfo != nil {
		processInfo = &ProcessInfo{
			ProcessID:   int64(conn.ProcessInfo.ProcessId),
			UserID:      conn.ProcessInfo.UserId,
			UserName:    conn.ProcessInfo.UserName,
			ProcessPath: conn.ProcessInfo.ProcessPath,
			PackageName: conn.ProcessInfo.PackageName,
		}
	}
	return Connection{
		ID:            conn.Id,
		Inbound:       conn.Inbound,
		InboundType:   conn.InboundType,
		IPVersion:     conn.IpVersion,
		Network:       conn.Network,
		Source:        conn.Source,
		Destination:   conn.Destination,
		Domain:        conn.Domain,
		Protocol:      conn.Protocol,
		User:          conn.User,
		FromOutbound:  conn.FromOutbound,
		CreatedAt:     conn.CreatedAt,
		ClosedAt:      conn.ClosedAt,
		Uplink:        conn.Uplink,
		Downlink:      conn.Downlink,
		UplinkTotal:   conn.UplinkTotal,
		DownlinkTotal: conn.DownlinkTotal,
		Rule:          conn.Rule,
		Outbound:      conn.Outbound,
		OutboundType:  conn.OutboundType,
		ChainList:     conn.ChainList,
		ProcessInfo:   processInfo,
	}
}

func ConnectionEventFromGRPC(event *daemon.ConnectionEvent) *ConnectionEvent {
	if event == nil {
		return nil
	}
	libboxEvent := &ConnectionEvent{
		Type:          int32(event.Type),
		ID:            event.Id,
		UplinkDelta:   event.UplinkDelta,
		DownlinkDelta: event.DownlinkDelta,
		ClosedAt:      event.ClosedAt,
	}
	if event.Connection != nil {
		conn := ConnectionFromGRPC(event.Connection)
		libboxEvent.Connection = &conn
	}
	return libboxEvent
}

func ConnectionEventsFromGRPC(events *daemon.ConnectionEvents) *ConnectionEvents {
	if events == nil {
		return nil
	}
	libboxEvents := &ConnectionEvents{
		Reset: events.Reset_,
	}
	for _, event := range events.Events {
		if libboxEvent := ConnectionEventFromGRPC(event); libboxEvent != nil {
			libboxEvents.events = append(libboxEvents.events, libboxEvent)
		}
	}
	return libboxEvents
}

func SystemProxyStatusFromGRPC(status *daemon.SystemProxyStatus) *SystemProxyStatus {
	if status == nil {
		return nil
	}
	return &SystemProxyStatus{
		Available: status.Available,
		Enabled:   status.Enabled,
	}
}

func SystemProxyStatusToGRPC(status *SystemProxyStatus) *daemon.SystemProxyStatus {
	if status == nil {
		return nil
	}
	return &daemon.SystemProxyStatus{
		Available: status.Available,
		Enabled:   status.Enabled,
	}
}
