package adapter

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/observable"
	"github.com/sagernet/sing/common/varbin"
)

type ClashServer interface {
	LifecycleService
	Mode() string
	ModeList() []string
	SetMode(mode string)
	AddModeUpdateHook(hook *observable.Subscriber[struct{}])
}

type URLTestHistory struct {
	Time time.Time `json:"time"`
	// Delay is the time to response headers, in milliseconds.
	Delay uint16 `json:"delay"`
	// Throughput is the smoothed effective transfer rate in bytes per second, or
	// zero when bandwidth testing is disabled or no sample has been taken yet.
	// It is a ranking signal measured over a few hundred KiB, not a speed test
	// result, and understates the capacity of a fast path.
	Throughput uint32 `json:"throughput,omitempty"`
	// Bytes is the number of body bytes read by the most recent bandwidth probe.
	Bytes uint32 `json:"bytes,omitempty"`
}

type V2RayServer interface {
	LifecycleService
	StatsService() ConnectionTracker
}

type CacheFile interface {
	LifecycleService

	CacheID() string

	StoreFakeIP() bool
	FakeIPStorage

	StoreRDRC() bool
	RDRCStore

	StoreDNS() bool
	DNSCacheStore

	SetDisableExpire(disableExpire bool)
	SetOptimisticTimeout(timeout time.Duration)

	LoadMode() string
	StoreMode(mode string) error
	LoadSelected(group string) string
	StoreSelected(group string, selected string) error
	LoadGroupExpand(group string) (isExpand bool, loaded bool)
	StoreGroupExpand(group string, expand bool) error
	LoadRuleSet(tag string) *SavedBinary
	SaveRuleSet(tag string, set *SavedBinary) error
}

type SavedBinary struct {
	Content     []byte
	LastUpdated time.Time
	LastEtag    string
	URLHash     []byte
}

func (s *SavedBinary) MarshalBinary() ([]byte, error) {
	var buffer bytes.Buffer
	err := binary.Write(&buffer, binary.BigEndian, uint8(2))
	if err != nil {
		return nil, err
	}
	_, err = varbin.WriteUvarint(&buffer, uint64(len(s.Content)))
	if err != nil {
		return nil, err
	}
	_, err = buffer.Write(s.Content)
	if err != nil {
		return nil, err
	}
	err = binary.Write(&buffer, binary.BigEndian, s.LastUpdated.Unix())
	if err != nil {
		return nil, err
	}
	_, err = varbin.WriteUvarint(&buffer, uint64(len(s.LastEtag)))
	if err != nil {
		return nil, err
	}
	_, err = buffer.WriteString(s.LastEtag)
	if err != nil {
		return nil, err
	}
	_, err = varbin.WriteUvarint(&buffer, uint64(len(s.URLHash)))
	if err != nil {
		return nil, err
	}
	_, err = buffer.Write(s.URLHash)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (s *SavedBinary) UnmarshalBinary(data []byte) error {
	reader := bytes.NewReader(data)
	var version uint8
	err := binary.Read(reader, binary.BigEndian, &version)
	if err != nil {
		return err
	}
	contentLength, err := binary.ReadUvarint(reader)
	if err != nil {
		return err
	}
	if contentLength > uint64(reader.Len()) {
		return E.New("invalid content length: ", contentLength)
	}
	s.Content = make([]byte, contentLength)
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
	etagLength, err := binary.ReadUvarint(reader)
	if err != nil {
		return err
	}
	if etagLength > uint64(reader.Len()) {
		return E.New("invalid etag length: ", etagLength)
	}
	etagBytes := make([]byte, etagLength)
	_, err = io.ReadFull(reader, etagBytes)
	if err != nil {
		return err
	}
	s.LastEtag = string(etagBytes)
	if version < 2 {
		return nil
	}
	urlHashLength, err := binary.ReadUvarint(reader)
	if err != nil {
		return err
	}
	if urlHashLength > uint64(reader.Len()) {
		return E.New("invalid url hash length: ", urlHashLength)
	}
	s.URLHash = make([]byte, urlHashLength)
	_, err = io.ReadFull(reader, s.URLHash)
	if err != nil {
		return err
	}
	return nil
}

type OutboundGroup interface {
	Outbound
	Now() string
	All() []string
}

type URLTestGroup interface {
	OutboundGroup
	URLTest(ctx context.Context) (map[string]uint16, error)
	PerformUpdateCheck()
}
