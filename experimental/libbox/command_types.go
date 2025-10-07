package libbox

import (
	"slices"
	"strings"

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

type Connections struct {
	input    []Connection
	filtered []Connection
}

func (c *Connections) FilterState(state int32) {
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
	}
}

func ConnectionsFromGRPC(connections *daemon.Connections) []Connection {
	if connections == nil || len(connections.Connections) == 0 {
		return nil
	}
	var libboxConnections []Connection
	for _, conn := range connections.Connections {
		libboxConnections = append(libboxConnections, ConnectionFromGRPC(conn))
	}
	return libboxConnections
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
