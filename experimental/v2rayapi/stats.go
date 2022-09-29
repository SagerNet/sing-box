package v2rayapi

import (
	"context"
	"net"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental/trackerconn"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"

	"go.uber.org/atomic"
)

func init() {
	StatsService_ServiceDesc.ServiceName = "v2ray.core.app.stats.command.StatsService"
}

var (
	_ adapter.V2RayStatsService = (*StatsService)(nil)
	_ StatsServiceServer        = (*StatsService)(nil)
)

type StatsService struct {
	createdAt time.Time
	directIO  bool
	inbounds  map[string]bool
	outbounds map[string]bool
	access    sync.Mutex
	counters  map[string]*atomic.Int64
}

func NewStatsService(options option.V2RayStatsServiceOptions) *StatsService {
	if !options.Enabled {
		return nil
	}
	inbounds := make(map[string]bool)
	outbounds := make(map[string]bool)
	for _, inbound := range options.Inbounds {
		inbounds[inbound] = true
	}
	for _, outbound := range options.Outbounds {
		outbounds[outbound] = true
	}
	return &StatsService{
		createdAt: time.Now(),
		directIO:  options.DirectIO,
		inbounds:  inbounds,
		outbounds: outbounds,
		counters:  make(map[string]*atomic.Int64),
	}
}

func (s *StatsService) RoutedConnection(inbound string, outbound string, conn net.Conn) net.Conn {
	var readCounter *atomic.Int64
	var writeCounter *atomic.Int64
	countInbound := inbound != "" && s.inbounds[inbound]
	countOutbound := outbound != "" && s.outbounds[outbound]
	if !countInbound && !countOutbound {
		return conn
	}
	s.access.Lock()
	if countInbound {
		readCounter = s.loadOrCreateCounter("inbound>>>"+inbound+">>>traffic>>>uplink", readCounter)
		writeCounter = s.loadOrCreateCounter("inbound>>>"+inbound+">>>traffic>>>downlink", writeCounter)
	}
	if countOutbound {
		readCounter = s.loadOrCreateCounter("outbound>>>"+outbound+">>>traffic>>>uplink", readCounter)
		writeCounter = s.loadOrCreateCounter("outbound>>>"+outbound+">>>traffic>>>downlink", writeCounter)
	}
	s.access.Unlock()
	return trackerconn.New(conn, readCounter, writeCounter, s.directIO)
}

func (s *StatsService) RoutedPacketConnection(inbound string, outbound string, conn N.PacketConn) N.PacketConn {
	var readCounter *atomic.Int64
	var writeCounter *atomic.Int64
	countInbound := inbound != "" && s.inbounds[inbound]
	countOutbound := outbound != "" && s.outbounds[outbound]
	if !countInbound && !countOutbound {
		return conn
	}
	s.access.Lock()
	if countInbound {
		readCounter = s.loadOrCreateCounter("inbound>>>"+inbound+">>>traffic>>>uplink", readCounter)
		writeCounter = s.loadOrCreateCounter("inbound>>>"+inbound+">>>traffic>>>downlink", writeCounter)
	}
	if countOutbound {
		readCounter = s.loadOrCreateCounter("outbound>>>"+outbound+">>>traffic>>>uplink", readCounter)
		writeCounter = s.loadOrCreateCounter("outbound>>>"+outbound+">>>traffic>>>downlink", writeCounter)
	}
	s.access.Unlock()
	return trackerconn.NewPacket(conn, readCounter, writeCounter)
}

func (s *StatsService) GetStats(ctx context.Context, request *GetStatsRequest) (*GetStatsResponse, error) {
	s.access.Lock()
	counter, loaded := s.counters[request.Name]
	s.access.Unlock()
	if !loaded {
		return nil, E.New(request.Name, " not found.")
	}
	var value int64
	if request.Reset_ {
		value = counter.Swap(0)
	} else {
		value = counter.Load()
	}
	return &GetStatsResponse{Stat: &Stat{Name: request.Name, Value: value}}, nil
}

func (s *StatsService) QueryStats(ctx context.Context, request *QueryStatsRequest) (*QueryStatsResponse, error) {
	var response QueryStatsResponse
	s.access.Lock()
	defer s.access.Unlock()
	if request.Regexp {
		matchers := make([]*regexp.Regexp, 0, len(request.Patterns))
		for _, pattern := range request.Patterns {
			matcher, err := regexp.Compile(pattern)
			if err != nil {
				return nil, err
			}
			matchers = append(matchers, matcher)
		}
		for name, counter := range s.counters {
			for _, matcher := range matchers {
				if matcher.MatchString(name) {
					var value int64
					if request.Reset_ {
						value = counter.Swap(0)
					} else {
						value = counter.Load()
					}
					response.Stat = append(response.Stat, &Stat{Name: name, Value: value})
				}
			}
		}
	} else {
		for name, counter := range s.counters {
			for _, matcher := range request.Patterns {
				if strings.Contains(name, matcher) {
					var value int64
					if request.Reset_ {
						value = counter.Swap(0)
					} else {
						value = counter.Load()
					}
					response.Stat = append(response.Stat, &Stat{Name: name, Value: value})
				}
			}
		}
	}
	return &response, nil
}

func (s *StatsService) GetSysStats(ctx context.Context, request *SysStatsRequest) (*SysStatsResponse, error) {
	var rtm runtime.MemStats
	runtime.ReadMemStats(&rtm)
	response := &SysStatsResponse{
		Uptime:       uint32(time.Now().Sub(s.createdAt).Seconds()),
		NumGoroutine: uint32(runtime.NumGoroutine()),
		Alloc:        rtm.Alloc,
		TotalAlloc:   rtm.TotalAlloc,
		Sys:          rtm.Sys,
		Mallocs:      rtm.Mallocs,
		Frees:        rtm.Frees,
		LiveObjects:  rtm.Mallocs - rtm.Frees,
		NumGC:        rtm.NumGC,
		PauseTotalNs: rtm.PauseTotalNs,
	}

	return response, nil
}

func (s *StatsService) mustEmbedUnimplementedStatsServiceServer() {
}

//nolint:staticcheck
func (s *StatsService) loadOrCreateCounter(name string, counter *atomic.Int64) *atomic.Int64 {
	counter, loaded := s.counters[name]
	if !loaded {
		if counter == nil {
			counter = atomic.NewInt64(0)
		}
		s.counters[name] = counter
	}
	return counter
}
