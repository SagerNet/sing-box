package dhcp

import (
	"context"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/dns/transport"
	E "github.com/sagernet/sing/common/exceptions"

	mDNS "github.com/miekg/dns"
)

func (t *Transport) exchangeWithTransports(ctx context.Context, message *mDNS.Msg, state *transportState, callback func(response *mDNS.Msg, err error)) {
	question := message.Question[0]
	names := nameList(state.search, question.Name)
	if len(names) == 0 {
		callback(nil, E.New("invalid domain: ", question.Name))
		return
	}
	transport.ExchangeNames(ctx, names, question, func(fqdn string) transport.AsyncExchanger {
		return t.newNameExchanger(message, fqdn, state.serverTransports)
	}, callback)
}

func (t *Transport) newNameExchanger(message *mDNS.Msg, fqdn string, serverTransports []adapter.DNSTransport) transport.AsyncExchanger {
	attemptExchangers := make([]transport.AsyncExchanger, 0, t.attempts*len(serverTransports))
	for range t.attempts {
		for _, serverTransport := range serverTransports {
			attemptExchangers = append(attemptExchangers, func(ctx context.Context, callback func(response *mDNS.Msg, err error)) {
				serverTransport.ExchangeAsync(ctx, transport.NewFanOutRequest(message, fqdn, true), callback)
			})
		}
	}
	return func(ctx context.Context, callback func(response *mDNS.Msg, err error)) {
		transport.ExchangeSequential(ctx, attemptExchangers, nil, func(response *mDNS.Msg, err error) {
			if err != nil {
				err = E.Cause(err, fqdn)
			}
			callback(response, err)
		})
	}
}

func nameList(search []string, name string) []string {
	if len(name) > 254 || avoidDNS(name) {
		return nil
	}
	if mDNS.CountLabel(name) != 1 {
		return []string{name}
	}
	names := make([]string, 0, len(search)+1)
	for _, suffix := range search {
		fqdn := name + suffix
		if !avoidDNS(fqdn) && len(fqdn) <= 254 {
			names = append(names, fqdn)
		}
	}
	return append(names, name)
}

func avoidDNS(name string) bool {
	if name == "" {
		return true
	}
	if name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	return strings.HasSuffix(name, ".onion")
}
