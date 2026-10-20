package local

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport"
	"github.com/sagernet/sing-box/dns/transport/local/systemconfig"
	E "github.com/sagernet/sing/common/exceptions"

	mDNS "github.com/miekg/dns"
)

type localServerSet struct {
	config      *systemconfig.Config
	transports  []adapter.DNSTransport
	serverScope *adapter.Scope
}

func (t *Transport) serverSetFor(systemConfig *systemconfig.Config) (*localServerSet, error) {
	serverSet := t.serverSet.Load()
	if serverSet != nil && serverSet.config == systemConfig {
		return serverSet, nil
	}
	t.serverSetAccess.Lock()
	defer t.serverSetAccess.Unlock()
	serverSet = t.serverSet.Load()
	if serverSet != nil && serverSet.config == systemConfig {
		return serverSet, nil
	}
	serverScope := adapter.NewScope(t.ctx, t.logger)
	transports := make([]adapter.DNSTransport, 0, len(systemConfig.Servers))
	for _, serverAddr := range systemConfig.Servers {
		var serverTransport adapter.DNSTransport
		if systemConfig.UseTCP {
			serverTransport = transport.NewTCPRaw(dns.NewTransportAdapter(C.DNSTypeTCP, "", nil), t.dialer, serverAddr)
		} else {
			serverTransport = transport.NewUDPRaw(t.logger, dns.NewTransportAdapter(C.DNSTypeUDP, "", nil), t.dialer, serverAddr)
		}
		err := serverTransport.Start(adapter.StartStateStart, serverScope)
		if err != nil {
			return nil, E.Errors(E.Cause(err, "initialize transport for ", serverAddr), serverScope.Close())
		}
		transports = append(transports, serverTransport)
	}
	newServerSet := &localServerSet{
		config:      systemConfig,
		transports:  transports,
		serverScope: serverScope,
	}
	oldServerSet := t.serverSet.Swap(newServerSet)
	if oldServerSet != nil {
		oldServerSet.serverScope.Close()
	}
	return newServerSet, nil
}

func (t *Transport) exchangeAsync(ctx context.Context, message *mDNS.Msg, systemConfig *systemconfig.Config, callback func(response *mDNS.Msg, err error)) {
	err := transport.ValidateQuestionName(message.Question[0].Name)
	if err != nil {
		callback(nil, err)
		return
	}
	serverSet, err := t.serverSetFor(systemConfig)
	if err != nil {
		callback(nil, err)
		return
	}
	serverOffset := systemConfig.ServerOffset()
	serverCount := uint32(len(serverSet.transports))
	attemptExchangers := make([]transport.AsyncExchanger, 0, systemConfig.Attempts*int(serverCount))
	for range systemConfig.Attempts {
		for j := range serverCount {
			serverTransport := serverSet.transports[(serverOffset+j)%serverCount]
			attemptExchangers = append(attemptExchangers, func(exchangeCtx context.Context, exchangeCallback func(response *mDNS.Msg, err error)) {
				attemptCtx, cancel := context.WithTimeout(exchangeCtx, systemConfig.Timeout)
				serverTransport.ExchangeAsync(attemptCtx, transport.NewFanOutRequest(message, message.Question[0].Name, systemConfig.TrustAD), func(response *mDNS.Msg, attemptErr error) {
					cancel()
					exchangeCallback(response, attemptErr)
				})
			})
		}
	}
	transport.ExchangeSequential(ctx, attemptExchangers, nil, callback)
}
