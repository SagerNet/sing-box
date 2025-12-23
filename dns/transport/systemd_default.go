package transport

import (
	"context"
	mDNS "github.com/miekg/dns"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	tun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/service"
	"os/exec"
	"strings"
	"sync"
)

type SystemdDefault struct {
	*UDPTransport
	unspecified   bool
	ifcMonitor    tun.DefaultInterfaceMonitor
	manualCheckMu sync.Mutex
}

func RegisterUnderlying(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.RemoteDNSServerOptions](registry, C.DNSTypeUnderlying, NewSystemdDefault)
}

func NewSystemdDefault(ctx context.Context, logger log.ContextLogger, tag string, options option.RemoteDNSServerOptions) (adapter.DNSTransport, error) {
	transportDialer, err := dns.NewRemoteDialer(ctx, options)
	if err != nil {
		return nil, err
	}
	transport := NewUDPRaw(logger, dns.NewTransportAdapterWithRemoteOptions(C.DNSTypeUDP, tag, options), transportDialer, metadata.ParseSocksaddr("0.0.0.0:0"))
	s := &SystemdDefault{UDPTransport: transport, unspecified: true}
	nm := service.FromContext[adapter.NetworkManager](ctx)
	nm.InterfaceMonitor().RegisterCallback(s.handleInterfaceUpdate)
	s.handleInterfaceUpdate(nm.InterfaceMonitor().DefaultInterface(), 0)
	s.ifcMonitor = nm.InterfaceMonitor()
	return s, nil
}

func (s *SystemdDefault) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	if !s.unspecified {
		return s.UDPTransport.Exchange(ctx, message)
	}
	s.manualCheckMu.Lock()
	if !s.unspecified {
		s.manualCheckMu.Unlock()
		return s.UDPTransport.Exchange(ctx, message)
	}
	defaultIfc := s.ifcMonitor.DefaultInterface()
	if defaultIfc == nil {
		s.manualCheckMu.Unlock()
		return nil, E.New("No default interface")
	}
	s.handleInterfaceUpdate(defaultIfc, 0)
	s.manualCheckMu.Unlock()
	return s.UDPTransport.Exchange(ctx, message)
}

func (s *SystemdDefault) handleInterfaceUpdate(defaultInterface *control.Interface, flags int) {
	failedFunc := func() {
		s.connector.access.Lock()
		s.connector.Reset()
		s.serverAddr = metadata.ParseSocksaddr("0.0.0.0:0")
		s.unspecified = true
		s.connector.access.Unlock()
	}

	if defaultInterface == nil {
		s.Logger.Error("No default interface")
		failedFunc()
		return
	}
	cmd := "resolvectl"
	args := []string{"-i", defaultInterface.Name, "dns"}
	res, err := exec.Command(cmd, args...).Output()
	if err != nil {
		s.Logger.Error("Could not call resolvectl ", err)
		failedFunc()
		return
	}
	server, err := s.parseResolvectlOutput(string(res))
	if err != nil {
		s.Logger.Error("failed to parse resolvectl output ", err)
		failedFunc()
		return
	}
	s.connector.access.Lock()
	s.connector.Reset()
	s.serverAddr = server
	s.unspecified = false
	s.Logger.Info("underlying dns set to ", server)
	s.connector.access.Unlock()
}

func (s *SystemdDefault) parseResolvectlOutput(out string) (metadata.Socksaddr, error) {
	spl := strings.Split(out, " ")
	if len(spl) < 4 {
		return metadata.Socksaddr{}, E.New("failed to parse resolvectl output: ", out)
	}
	return metadata.ParseSocksaddr(strings.TrimSpace(spl[3]) + ":53"), nil
}
