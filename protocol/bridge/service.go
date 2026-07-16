//go:build linux || darwin

package bridge

import (
	"net/netip"
	"os"
	"sync"

	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/x/list"
)

type serviceBase struct {
	logger            logger.ContextLogger
	mtu               int
	inet4Port         netip.Addr
	inet6Port         netip.Addr
	tunName           string
	tunFileDescriptor int
	forwardingRestore []sysctlState

	networkMonitor tun.NetworkUpdateMonitor
	monitorElement *list.Element[tun.NetworkUpdateCallback]

	access      sync.Mutex
	egressName  string
	closed      bool
	applyEgress func() error
}

func (s *serviceBase) FileDescriptor() int {
	return s.tunFileDescriptor
}

func (s *serviceBase) Name() string {
	return s.tunName
}

func (s *serviceBase) Inet6Active() bool {
	return s.inet6Port.IsValid()
}

func (s *serviceBase) SetEgress(interfaceName string) error {
	s.access.Lock()
	defer s.access.Unlock()
	if s.closed {
		return os.ErrClosed
	}
	s.egressName = interfaceName
	return s.applyEgress()
}

func (s *serviceBase) syncEgress() {
	s.access.Lock()
	defer s.access.Unlock()
	if s.closed {
		return
	}
	err := s.applyEgress()
	if err != nil {
		s.logger.Debug(E.Cause(err, "update bridge egress"))
	}
}

func (s *serviceBase) startNetworkMonitor() {
	networkMonitor, err := tun.NewNetworkUpdateMonitor(s.logger)
	if err != nil {
		s.logger.Debug(E.Cause(err, "create network monitor, egress will not track route changes"))
		return
	}
	s.monitorElement = networkMonitor.RegisterCallback(func() { s.syncEgress() })
	s.networkMonitor = networkMonitor
	err = networkMonitor.Start()
	if err != nil {
		s.logger.Debug(E.Cause(err, "start network monitor, egress will not track route changes"))
	}
}

func (s *serviceBase) beginClose() bool {
	s.access.Lock()
	if s.closed {
		s.access.Unlock()
		return false
	}
	s.closed = true
	networkMonitor := s.networkMonitor
	monitorElement := s.monitorElement
	s.networkMonitor = nil
	s.monitorElement = nil
	s.access.Unlock()
	if networkMonitor != nil {
		if monitorElement != nil {
			networkMonitor.UnregisterCallback(monitorElement)
		}
		_ = networkMonitor.Close()
	}
	return true
}
