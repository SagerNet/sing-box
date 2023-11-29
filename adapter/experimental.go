package adapter

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"time"

	"github.com/sagernet/sing-box/common/urltest"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
)

type ClashServer interface {
	Service
	PreStarter
	Mode() string
	ModeList() []string
	HistoryStorage() *urltest.HistoryStorage
	RoutedConnection(ctx context.Context, conn net.Conn, metadata InboundContext, matchedRule Rule) (net.Conn, Tracker)
	RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata InboundContext, matchedRule Rule) (N.PacketConn, Tracker)
}

type CacheFile interface {
	Service
	PreStarter

	StoreFakeIP() bool
	FakeIPStorage

	LoadMode() string
	StoreMode(mode string) error
	LoadSelected(group string) string
	StoreSelected(group string, selected string) error
	LoadGroupExpand(group string) (isExpand bool, loaded bool)
	StoreGroupExpand(group string, expand bool) error
	LoadRuleSet(tag string) *SavedRuleSet
	SaveRuleSet(tag string, set *SavedRuleSet) error
}

type SavedRuleSet struct {
	Content     []byte
	LastUpdated time.Time
	LastEtag    string
}

func (s *SavedRuleSet) MarshalBinary() ([]byte, error) {
	var buffer bytes.Buffer
	err := binary.Write(&buffer, binary.BigEndian, uint8(1))
	if err != nil {
		return nil, err
	}
	err = rw.WriteUVariant(&buffer, uint64(len(s.Content)))
	if err != nil {
		return nil, err
	}
	buffer.Write(s.Content)
	err = binary.Write(&buffer, binary.BigEndian, s.LastUpdated.Unix())
	if err != nil {
		return nil, err
	}
	err = rw.WriteVString(&buffer, s.LastEtag)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (s *SavedRuleSet) UnmarshalBinary(data []byte) error {
	reader := bytes.NewReader(data)
	var version uint8
	err := binary.Read(reader, binary.BigEndian, &version)
	if err != nil {
		return err
	}
	contentLen, err := rw.ReadUVariant(reader)
	if err != nil {
		return err
	}
	s.Content = make([]byte, contentLen)
	_, err = io.ReadFull(reader, s.Content)
	if err != nil {
		return err
	}
	var lastUpdated int64
	err = binary.Read(reader, binary.BigEndian, &lastUpdated)
	if err != nil {
		return err
	}
	s.LastUpdated = time.Unix(lastUpdated, 0)
	s.LastEtag, err = rw.ReadVString(reader)
	if err != nil {
		return err
	}
	return nil
}

type Tracker interface {
	Leave()
}

type OutboundGroup interface {
	Outbound
	Now() string
	All() []string
}

type URLTestGroup interface {
	OutboundGroup
	URLTest(ctx context.Context, url string) (map[string]uint16, error)
}

func OutboundTag(detour Outbound) string {
	if group, isGroup := detour.(OutboundGroup); isGroup {
		return group.Now()
	}
	return detour.Tag()
}

type V2RayServer interface {
	Service
	StatsService() V2RayStatsService
}

type V2RayStatsService interface {
	RoutedConnection(inbound string, outbound string, user string, conn net.Conn) net.Conn
	RoutedPacketConnection(inbound string, outbound string, user string, conn N.PacketConn) N.PacketConn
}
