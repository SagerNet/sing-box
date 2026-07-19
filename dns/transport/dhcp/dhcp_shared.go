package dhcp

import (
	"context"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport"
	E "github.com/sagernet/sing/common/exceptions"

	mDNS "github.com/miekg/dns"
)

func (t *Transport) exchangeWithTransports(ctx context.Context, message *mDNS.Msg, serverTransports []adapter.DNSTransport, callback func(response *mDNS.Msg, err error)) {
	question := message.Question[0]
	domain := dns.FqdnToDomain(question.Name)
	names := t.nameList(domain)
	if len(names) == 0 {
		callback(nil, E.New("invalid domain: ", domain))
		return
	}
	nameExchangers := make([]transport.AsyncExchanger, 0, len(names))
	for _, fqdn := range names {
		nameExchangers = append(nameExchangers, t.newNameExchanger(message, fqdn, serverTransports))
	}
	if len(serverTransports) == 1 || !(question.Qtype == mDNS.TypeA || question.Qtype == mDNS.TypeAAAA) {
		transport.ExchangeSequential(ctx, nameExchangers, nil, callback)
	} else {
		transport.ExchangeRace(ctx, nameExchangers, callback)
	}
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

func (t *Transport) nameList(name string) []string {
	l := len(name)
	rooted := l > 0 && name[l-1] == '.'
	if l > 254 || l == 254 && !rooted {
		return nil
	}

	if rooted {
		if avoidDNS(name) {
			return nil
		}
		return []string{name}
	}

	hasNdots := strings.Count(name, ".") >= t.ndots
	name += "."
	// l++

	names := make([]string, 0, 1+len(t.search))
	if hasNdots && !avoidDNS(name) {
		names = append(names, name)
	}
	for _, suffix := range t.search {
		fqdn := name + suffix
		if !avoidDNS(fqdn) && len(fqdn) <= 254 {
			names = append(names, fqdn)
		}
	}
	if !hasNdots && !avoidDNS(name) {
		names = append(names, name)
	}
	return names
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
