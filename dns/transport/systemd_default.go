package transport

import (
	"context"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/metadata"
//	"github.com/sagernet/sing/service"
	"strings"
)

type SystemdDefault struct {
	*UDPTransport
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
	s := &SystemdDefault{UDPTransport: transport}
//	nm := service.FromContext[adapter.NetworkManager](ctx)
//	nm.InterfaceMonitor().RegisterCallback(s.handleInterfaceUpdate)
//	s.handleInterfaceUpdate(nm.InterfaceMonitor().DefaultInterface(), 0)
	return s, nil
}

func (s *SystemdDefault) parseResolvectlOutput(out string) (metadata.Socksaddr, error) {
	spl := strings.Split(out, " ")
	if len(out) < 4 {
		return metadata.Socksaddr{}, E.New("failed to parse resolvectl output: ", out)
	}
	return metadata.ParseSocksaddr(spl[3] + ":53"), nil
}
