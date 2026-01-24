package v2rayapi

import (
	"context"
	"net"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

func init() {
	StatsService_ServiceDesc.ServiceName = "v2ray.core.app.stats.command.StatsService"
}

var (
	_ adapter.ConnectionTracker = (*StatsService)(nil)
	_ StatsServiceServer        = (*StatsService)(nil)
)

type StatsService struct {
	createdAt time.Time
	inbounds  map[string]bool
	outbounds map[string]bool
	users     map[string]bool
	access    sync.Mutex
	counters  map[string]*atomic.Int64
}

func NewStatsService(options option.V2RayStatsServiceOptions) *StatsService {
	if !options.Enabled {
		return nil
	}
	inbounds := make(map[string]bool)
	outbounds := make(map[string]bool)
	users := make(map[string]bool)
	for _, inbound := range options.Inbounds {
		inbounds[inbound] = true
	}
	for _, outbound := range options.Outbounds {
		outbounds[outbound] = true
	}
	for _, user := range options.Users {
		users[user] = true
	}
	return &StatsService{
		createdAt: time.Now(),
		inbounds:  inbounds,
		outbounds: outbounds,
		users:     users,
		counters:  make(map[string]*atomic.Int64),
	}
}

func (s *StatsService) RoutedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) net.Conn {
	inbound := metadata.Inbound
	user := metadata.User
	outbound := matchOutbound.Tag()
	var readCounter []*atomic.Int64
	var writeCounter []*atomic.Int64
	countInbound := inbound != "" && s.inbounds[inbound]
	countOutbound := outbound != "" && s.outbounds[outbound]
	countUser := user != "" && s.users[user]
	if !countInbound && !countOutbound && !countUser {
		return conn
	}
	s.access.Lock()
	if countInbound {
		readCounter = append(readCounter, s.loadOrCreateCounter("inbound>>>"+inbound+">>>traffic>>>uplink"))
		writeCounter = append(writeCounter, s.loadOrCreateCounter("inbound>>>"+inbound+">>>traffic>>>downlink"))
	}
	if countOutbound {
		readCounter = append(readCounter, s.loadOrCreateCounter("outbound>>>"+outbound+">>>traffic>>>uplink"))
		writeCounter = append(writeCounter, s.loadOrCreateCounter("outbound>>>"+outbound+">>>traffic>>>downlink"))
	}
	if countUser {
		readCounter = append(readCounter, s.loadOrCreateCounter("user>>>"+user+">>>traffic>>>uplink"))
		writeCounter = append(writeCounter, s.loadOrCreateCounter("user>>>"+user+">>>traffic>>>downlink"))
	}
	s.access.Unlock()
	return bufio.NewInt64CounterConn(conn, readCounter, writeCounter)
}

func (s *StatsService) RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) N.PacketConn {
	inbound := metadata.Inbound
	user := metadata.User
	outbound := matchOutbound.Tag()
	var readCounter []*atomic.Int64
	var writeCounter []*atomic.Int64
	countInbound := inbound != "" && s.inbounds[inbound]
	countOutbound := outbound != "" && s.outbounds[outbound]
	countUser := user != "" && s.users[user]
	if !countInbound && !countOutbound && !countUser {
		return conn
	}
	s.access.Lock()
	if countInbound {
		readCounter = append(readCounter, s.loadOrCreateCounter("inbound>>>"+inbound+">>>traffic>>>uplink"))
		writeCounter = append(writeCounter, s.loadOrCreateCounter("inbound>>>"+inbound+">>>traffic>>>downlink"))
	}
	if countOutbound {
		readCounter = append(readCounter, s.loadOrCreateCounter("outbound>>>"+outbound+">>>traffic>>>uplink"))
		writeCounter = append(writeCounter, s.loadOrCreateCounter("outbound>>>"+outbound+">>>traffic>>>downlink"))
	}
	if countUser {
		readCounter = append(readCounter, s.loadOrCreateCounter("user>>>"+user+">>>traffic>>>uplink"))
		writeCounter = append(writeCounter, s.loadOrCreateCounter("user>>>"+user+">>>traffic>>>downlink"))
	}
	s.access.Unlock()
	return bufio.NewInt64CounterPacketConn(conn, readCounter, nil, writeCounter, nil)
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
	if len(request.Patterns) == 0 {
		for name, counter := range s.counters {
			var value int64
			if request.Reset_ {
				value = counter.Swap(0)
			} else {
				value = counter.Load()
			}
			response.Stat = append(response.Stat, &Stat{Name: name, Value: value})
		}
	} else if request.Regexp {
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
		Uptime:       uint32(time.Since(s.createdAt).Seconds()),
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
func (s *StatsService) loadOrCreateCounter(name string) *atomic.Int64 {
	counter, loaded := s.counters[name]
	if loaded {
		return counter
	}
	counter = &atomic.Int64{}
	s.counters[name] = counter
	return counter
}
