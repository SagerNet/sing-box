package ssmapi

import (
	"net"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental/trackerconn"
	N "github.com/sagernet/sing/common/network"

	"go.uber.org/atomic"
)

type TrafficManager struct {
	nodeTags              map[string]bool
	nodeUsers             map[string]bool
	globalUplink          *atomic.Int64
	globalDownlink        *atomic.Int64
	globalUplinkPackets   *atomic.Int64
	globalDownlinkPackets *atomic.Int64
	globalTCPSessions     *atomic.Int64
	globalUDPSessions     *atomic.Int64
	userAccess            sync.Mutex
	userUplink            map[string]*atomic.Int64
	userDownlink          map[string]*atomic.Int64
	userUplinkPackets     map[string]*atomic.Int64
	userDownlinkPackets   map[string]*atomic.Int64
	userTCPSessions       map[string]*atomic.Int64
	userUDPSessions       map[string]*atomic.Int64
}

func NewTrafficManager(nodes []Node) *TrafficManager {
	manager := &TrafficManager{
		nodeTags:              make(map[string]bool),
		globalUplink:          atomic.NewInt64(0),
		globalDownlink:        atomic.NewInt64(0),
		globalUplinkPackets:   atomic.NewInt64(0),
		globalDownlinkPackets: atomic.NewInt64(0),
		globalTCPSessions:     atomic.NewInt64(0),
		globalUDPSessions:     atomic.NewInt64(0),
		userUplink:            make(map[string]*atomic.Int64),
		userDownlink:          make(map[string]*atomic.Int64),
		userUplinkPackets:     make(map[string]*atomic.Int64),
		userDownlinkPackets:   make(map[string]*atomic.Int64),
		userTCPSessions:       make(map[string]*atomic.Int64),
		userUDPSessions:       make(map[string]*atomic.Int64),
	}
	for _, node := range nodes {
		manager.nodeTags[node.Tag()] = true
	}
	return manager
}

func (s *TrafficManager) UpdateUsers(users []string) {
	nodeUsers := make(map[string]bool)
	for _, user := range users {
		nodeUsers[user] = true
	}
	s.nodeUsers = nodeUsers
}

func (s *TrafficManager) userCounter(user string) (*atomic.Int64, *atomic.Int64, *atomic.Int64, *atomic.Int64, *atomic.Int64, *atomic.Int64) {
	s.userAccess.Lock()
	defer s.userAccess.Unlock()
	upCounter, loaded := s.userUplink[user]
	if !loaded {
		upCounter = atomic.NewInt64(0)
		s.userUplink[user] = upCounter
	}
	downCounter, loaded := s.userDownlink[user]
	if !loaded {
		downCounter = atomic.NewInt64(0)
		s.userDownlink[user] = downCounter
	}
	upPacketsCounter, loaded := s.userUplinkPackets[user]
	if !loaded {
		upPacketsCounter = atomic.NewInt64(0)
		s.userUplinkPackets[user] = upPacketsCounter
	}
	downPacketsCounter, loaded := s.userDownlinkPackets[user]
	if !loaded {
		downPacketsCounter = atomic.NewInt64(0)
		s.userDownlinkPackets[user] = downPacketsCounter
	}
	tcpSessionsCounter, loaded := s.userTCPSessions[user]
	if !loaded {
		tcpSessionsCounter = atomic.NewInt64(0)
		s.userTCPSessions[user] = tcpSessionsCounter
	}
	udpSessionsCounter, loaded := s.userUDPSessions[user]
	if !loaded {
		udpSessionsCounter = atomic.NewInt64(0)
		s.userUDPSessions[user] = udpSessionsCounter
	}
	return upCounter, downCounter, upPacketsCounter, downPacketsCounter, tcpSessionsCounter, udpSessionsCounter
}

func createCounter(counterList []*atomic.Int64, packetCounterList []*atomic.Int64) func(n int64) {
	return func(n int64) {
		for _, counter := range counterList {
			counter.Add(n)
		}
		for _, counter := range packetCounterList {
			counter.Inc()
		}
	}
}

func (s *TrafficManager) RoutedConnection(metadata adapter.InboundContext, conn net.Conn) net.Conn {
	s.globalTCPSessions.Inc()

	var readCounter []*atomic.Int64
	var writeCounter []*atomic.Int64

	if s.nodeTags[metadata.Inbound] {
		readCounter = append(readCounter, s.globalUplink)
		writeCounter = append(writeCounter, s.globalDownlink)
	}
	if s.nodeUsers[metadata.User] {
		upCounter, downCounter, _, _, tcpSessionCounter, _ := s.userCounter(metadata.User)
		readCounter = append(readCounter, upCounter)
		writeCounter = append(writeCounter, downCounter)
		tcpSessionCounter.Inc()
	}
	if len(readCounter) > 0 {
		return trackerconn.New(conn, readCounter, writeCounter)
	}
	return conn
}

func (s *TrafficManager) RoutedPacketConnection(metadata adapter.InboundContext, conn N.PacketConn) N.PacketConn {
	s.globalUDPSessions.Inc()

	var readCounter []*atomic.Int64
	var readPacketCounter []*atomic.Int64
	var writeCounter []*atomic.Int64
	var writePacketCounter []*atomic.Int64

	if s.nodeTags[metadata.Inbound] {
		readCounter = append(readCounter, s.globalUplink)
		writeCounter = append(writeCounter, s.globalDownlink)
		readPacketCounter = append(readPacketCounter, s.globalUplinkPackets)
		writePacketCounter = append(writePacketCounter, s.globalDownlinkPackets)
	}
	if s.nodeUsers[metadata.User] {
		upCounter, downCounter, upPacketsCounter, downPacketsCounter, _, udpSessionCounter := s.userCounter(metadata.User)
		readCounter = append(readCounter, upCounter)
		writeCounter = append(writeCounter, downCounter)
		readPacketCounter = append(readPacketCounter, upPacketsCounter)
		writePacketCounter = append(writePacketCounter, downPacketsCounter)
		udpSessionCounter.Inc()
	}
	if len(readCounter) > 0 {
		return trackerconn.NewHookPacket(conn, createCounter(readCounter, readPacketCounter), createCounter(writeCounter, writePacketCounter))
	}
	return conn
}

func (s *TrafficManager) ReadUser(user *SSMUserObject) {
	s.userAccess.Lock()
	defer s.userAccess.Unlock()

	s.readUser(user)
}

func (s *TrafficManager) readUser(user *SSMUserObject) {
	if counter, loaded := s.userUplink[user.UserName]; loaded {
		user.UplinkBytes = counter.Load()
	}
	if counter, loaded := s.userDownlink[user.UserName]; loaded {
		user.DownlinkBytes = counter.Load()
	}
	if counter, loaded := s.userUplinkPackets[user.UserName]; loaded {
		user.UplinkPackets = counter.Load()
	}
	if counter, loaded := s.userDownlinkPackets[user.UserName]; loaded {
		user.DownlinkPackets = counter.Load()
	}
	if counter, loaded := s.userTCPSessions[user.UserName]; loaded {
		user.TCPSessions = counter.Load()
	}
	if counter, loaded := s.userUDPSessions[user.UserName]; loaded {
		user.UDPSessions = counter.Load()
	}
}

func (s *TrafficManager) ReadUsers(users []*SSMUserObject) {
	s.userAccess.Lock()
	defer s.userAccess.Unlock()
	for _, user := range users {
		s.readUser(user)
	}
	return
}

func (s *TrafficManager) ReadGlobal() (
	uplinkBytes int64,
	downlinkBytes int64,
	uplinkPackets int64,
	downlinkPackets int64,
	tcpSessions int64,
	udpSessions int64,
) {
	return s.globalUplink.Load(),
		s.globalDownlink.Load(),
		s.globalUplinkPackets.Load(),
		s.globalDownlinkPackets.Load(),
		s.globalTCPSessions.Load(),
		s.globalUDPSessions.Load()
}

func (s *TrafficManager) Clear() {
	s.globalUplink.Store(0)
	s.globalDownlink.Store(0)
	s.globalUplinkPackets.Store(0)
	s.globalDownlinkPackets.Store(0)
	s.globalTCPSessions.Store(0)
	s.globalUDPSessions.Store(0)
	s.userAccess.Lock()
	defer s.userAccess.Unlock()
	s.userUplink = make(map[string]*atomic.Int64)
	s.userDownlink = make(map[string]*atomic.Int64)
	s.userUplinkPackets = make(map[string]*atomic.Int64)
	s.userDownlinkPackets = make(map[string]*atomic.Int64)
	s.userTCPSessions = make(map[string]*atomic.Int64)
	s.userUDPSessions = make(map[string]*atomic.Int64)
}
