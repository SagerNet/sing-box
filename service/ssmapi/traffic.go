package ssmapi

import (
	"net"
	"sync"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.SSMTracker = (*TrafficManager)(nil)

type TrafficManager struct {
	globalUplink          atomic.Int64
	globalDownlink        atomic.Int64
	globalUplinkPackets   atomic.Int64
	globalDownlinkPackets atomic.Int64
	globalTCPSessions     atomic.Int64
	globalUDPSessions     atomic.Int64
	userAccess            sync.Mutex
	userUplink            map[string]*atomic.Int64
	userDownlink          map[string]*atomic.Int64
	userUplinkPackets     map[string]*atomic.Int64
	userDownlinkPackets   map[string]*atomic.Int64
	userTCPSessions       map[string]*atomic.Int64
	userUDPSessions       map[string]*atomic.Int64
}

func NewTrafficManager() *TrafficManager {
	manager := &TrafficManager{
		userUplink:          make(map[string]*atomic.Int64),
		userDownlink:        make(map[string]*atomic.Int64),
		userUplinkPackets:   make(map[string]*atomic.Int64),
		userDownlinkPackets: make(map[string]*atomic.Int64),
		userTCPSessions:     make(map[string]*atomic.Int64),
		userUDPSessions:     make(map[string]*atomic.Int64),
	}
	return manager
}

func (s *TrafficManager) UpdateUsers(users []string) {
	s.userAccess.Lock()
	defer s.userAccess.Unlock()
	newUserUplink := make(map[string]*atomic.Int64)
	newUserDownlink := make(map[string]*atomic.Int64)
	newUserUplinkPackets := make(map[string]*atomic.Int64)
	newUserDownlinkPackets := make(map[string]*atomic.Int64)
	newUserTCPSessions := make(map[string]*atomic.Int64)
	newUserUDPSessions := make(map[string]*atomic.Int64)
	for _, user := range users {
		if counter, loaded := s.userUplink[user]; loaded {
			newUserUplink[user] = counter
		}
		if counter, loaded := s.userDownlink[user]; loaded {
			newUserDownlink[user] = counter
		}
		if counter, loaded := s.userUplinkPackets[user]; loaded {
			newUserUplinkPackets[user] = counter
		}
		if counter, loaded := s.userDownlinkPackets[user]; loaded {
			newUserDownlinkPackets[user] = counter
		}
		if counter, loaded := s.userTCPSessions[user]; loaded {
			newUserTCPSessions[user] = counter
		}
		if counter, loaded := s.userUDPSessions[user]; loaded {
			newUserUDPSessions[user] = counter
		}
	}
	s.userUplink = newUserUplink
	s.userDownlink = newUserDownlink
	s.userUplinkPackets = newUserUplinkPackets
	s.userDownlinkPackets = newUserDownlinkPackets
	s.userTCPSessions = newUserTCPSessions
	s.userUDPSessions = newUserUDPSessions
}

func (s *TrafficManager) userCounter(user string) (*atomic.Int64, *atomic.Int64, *atomic.Int64, *atomic.Int64, *atomic.Int64, *atomic.Int64) {
	s.userAccess.Lock()
	defer s.userAccess.Unlock()
	upCounter, loaded := s.userUplink[user]
	if !loaded {
		upCounter = new(atomic.Int64)
		s.userUplink[user] = upCounter
	}
	downCounter, loaded := s.userDownlink[user]
	if !loaded {
		downCounter = new(atomic.Int64)
		s.userDownlink[user] = downCounter
	}
	upPacketsCounter, loaded := s.userUplinkPackets[user]
	if !loaded {
		upPacketsCounter = new(atomic.Int64)
		s.userUplinkPackets[user] = upPacketsCounter
	}
	downPacketsCounter, loaded := s.userDownlinkPackets[user]
	if !loaded {
		downPacketsCounter = new(atomic.Int64)
		s.userDownlinkPackets[user] = downPacketsCounter
	}
	tcpSessionsCounter, loaded := s.userTCPSessions[user]
	if !loaded {
		tcpSessionsCounter = new(atomic.Int64)
		s.userTCPSessions[user] = tcpSessionsCounter
	}
	udpSessionsCounter, loaded := s.userUDPSessions[user]
	if !loaded {
		udpSessionsCounter = new(atomic.Int64)
		s.userUDPSessions[user] = udpSessionsCounter
	}
	return upCounter, downCounter, upPacketsCounter, downPacketsCounter, tcpSessionsCounter, udpSessionsCounter
}

func (s *TrafficManager) TrackConnection(conn net.Conn, metadata adapter.InboundContext) net.Conn {
	s.globalTCPSessions.Add(1)
	var readCounter []*atomic.Int64
	var writeCounter []*atomic.Int64
	readCounter = append(readCounter, &s.globalUplink)
	writeCounter = append(writeCounter, &s.globalDownlink)
	upCounter, downCounter, _, _, tcpSessionCounter, _ := s.userCounter(metadata.User)
	readCounter = append(readCounter, upCounter)
	writeCounter = append(writeCounter, downCounter)
	tcpSessionCounter.Add(1)
	return bufio.NewInt64CounterConn(conn, readCounter, writeCounter)
}

func (s *TrafficManager) TrackPacketConnection(conn N.PacketConn, metadata adapter.InboundContext) N.PacketConn {
	s.globalUDPSessions.Add(1)
	var readCounter []*atomic.Int64
	var readPacketCounter []*atomic.Int64
	var writeCounter []*atomic.Int64
	var writePacketCounter []*atomic.Int64
	readCounter = append(readCounter, &s.globalUplink)
	writeCounter = append(writeCounter, &s.globalDownlink)
	readPacketCounter = append(readPacketCounter, &s.globalUplinkPackets)
	writePacketCounter = append(writePacketCounter, &s.globalDownlinkPackets)
	upCounter, downCounter, upPacketsCounter, downPacketsCounter, _, udpSessionCounter := s.userCounter(metadata.User)
	readCounter = append(readCounter, upCounter)
	writeCounter = append(writeCounter, downCounter)
	readPacketCounter = append(readPacketCounter, upPacketsCounter)
	writePacketCounter = append(writePacketCounter, downPacketsCounter)
	udpSessionCounter.Add(1)
	return bufio.NewInt64CounterPacketConn(conn, readCounter, readPacketCounter, writeCounter, writePacketCounter)
}

func (s *TrafficManager) ReadUser(user *UserObject) {
	s.userAccess.Lock()
	defer s.userAccess.Unlock()
	s.readUser(user, false)
}

func (s *TrafficManager) readUser(user *UserObject, swap bool) {
	if counter, loaded := s.userUplink[user.UserName]; loaded {
		if swap {
			user.UplinkBytes = counter.Swap(0)
		} else {
			user.UplinkBytes = counter.Load()
		}
	}
	if counter, loaded := s.userDownlink[user.UserName]; loaded {
		if swap {
			user.DownlinkBytes = counter.Swap(0)
		} else {
			user.DownlinkBytes = counter.Load()
		}
	}
	if counter, loaded := s.userUplinkPackets[user.UserName]; loaded {
		if swap {
			user.UplinkPackets = counter.Swap(0)
		} else {
			user.UplinkPackets = counter.Load()
		}
	}
	if counter, loaded := s.userDownlinkPackets[user.UserName]; loaded {
		if swap {
			user.DownlinkPackets = counter.Swap(0)
		} else {
			user.DownlinkPackets = counter.Load()
		}
	}
	if counter, loaded := s.userTCPSessions[user.UserName]; loaded {
		if swap {
			user.TCPSessions = counter.Swap(0)
		} else {
			user.TCPSessions = counter.Load()
		}
	}
	if counter, loaded := s.userUDPSessions[user.UserName]; loaded {
		if swap {
			user.UDPSessions = counter.Swap(0)
		} else {
			user.UDPSessions = counter.Load()
		}
	}
}

func (s *TrafficManager) ReadUsers(users []*UserObject, swap bool) {
	s.userAccess.Lock()
	defer s.userAccess.Unlock()
	for _, user := range users {
		s.readUser(user, swap)
	}
}

func (s *TrafficManager) ReadGlobal(swap bool) (uplinkBytes int64, downlinkBytes int64, uplinkPackets int64, downlinkPackets int64, tcpSessions int64, udpSessions int64) {
	if swap {
		return s.globalUplink.Swap(0),
			s.globalDownlink.Swap(0),
			s.globalUplinkPackets.Swap(0),
			s.globalDownlinkPackets.Swap(0),
			s.globalTCPSessions.Swap(0),
			s.globalUDPSessions.Swap(0)
	} else {
		return s.globalUplink.Load(),
			s.globalDownlink.Load(),
			s.globalUplinkPackets.Load(),
			s.globalDownlinkPackets.Load(),
			s.globalTCPSessions.Load(),
			s.globalUDPSessions.Load()
	}
}
