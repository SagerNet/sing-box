package local

import (
	"context"
	"errors"
	"net"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/experimental/libbox/platform"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
)

func RegisterTransport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.LocalDNSServerOptions](registry, C.DNSTypeLocal, NewFallbackTransport)
}

type FallbackTransport struct {
	adapter.DNSTransport
	ctx      context.Context
	fallback bool
	resolver net.Resolver
}

func NewFallbackTransport(ctx context.Context, logger log.ContextLogger, tag string, options option.LocalDNSServerOptions) (adapter.DNSTransport, error) {
	transport, err := NewTransport(ctx, logger, tag, options)
	if err != nil {
		return nil, err
	}
	return &FallbackTransport{
		DNSTransport: transport,
		ctx:          ctx,
	}, nil
}

func (f *FallbackTransport) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	platformInterface := service.FromContext[platform.Interface](f.ctx)
	if platformInterface == nil {
		return nil
	}
	inboundManager := service.FromContext[adapter.InboundManager](f.ctx)
	for _, inbound := range inboundManager.Inbounds() {
		if inbound.Type() == C.TypeTun {
			// platform tun hijacks DNS, so we can only use cgo resolver here
			f.fallback = true
			break
		}
	}
	return nil
}

func (f *FallbackTransport) Close() error {
	return nil
}

func (f *FallbackTransport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	if !f.fallback {
		return f.DNSTransport.Exchange(ctx, message)
	}
	question := message.Question[0]
	domain := dns.FqdnToDomain(question.Name)
	if question.Qtype == mDNS.TypeA || question.Qtype == mDNS.TypeAAAA {
		var network string
		if question.Qtype == mDNS.TypeA {
			network = "ip4"
		} else {
			network = "ip6"
		}
		addresses, err := f.resolver.LookupNetIP(ctx, network, domain)
		if err != nil {
			var dnsError *net.DNSError
			if errors.As(err, &dnsError) && dnsError.IsNotFound {
				return nil, dns.RcodeRefused
			}
			return nil, err
		}
		return dns.FixedResponse(message.Id, question, addresses, C.DefaultDNSTTL), nil
	} else if question.Qtype == mDNS.TypeNS {
		records, err := f.resolver.LookupNS(ctx, domain)
		if err != nil {
			var dnsError *net.DNSError
			if errors.As(err, &dnsError) && dnsError.IsNotFound {
				return nil, dns.RcodeRefused
			}
			return nil, err
		}
		response := &mDNS.Msg{
			MsgHdr: mDNS.MsgHdr{
				Id:       message.Id,
				Rcode:    mDNS.RcodeSuccess,
				Response: true,
			},
			Question: []mDNS.Question{question},
		}
		for _, record := range records {
			response.Answer = append(response.Answer, &mDNS.NS{
				Hdr: mDNS.RR_Header{
					Name:   question.Name,
					Rrtype: mDNS.TypeNS,
					Class:  mDNS.ClassINET,
					Ttl:    C.DefaultDNSTTL,
				},
				Ns: record.Host,
			})
		}
		return response, nil
	} else if question.Qtype == mDNS.TypeCNAME {
		cname, err := f.resolver.LookupCNAME(ctx, domain)
		if err != nil {
			var dnsError *net.DNSError
			if errors.As(err, &dnsError) && dnsError.IsNotFound {
				return nil, dns.RcodeRefused
			}
			return nil, err
		}
		return &mDNS.Msg{
			MsgHdr: mDNS.MsgHdr{
				Id:       message.Id,
				Rcode:    mDNS.RcodeSuccess,
				Response: true,
			},
			Question: []mDNS.Question{question},
			Answer: []mDNS.RR{
				&mDNS.CNAME{
					Hdr: mDNS.RR_Header{
						Name:   question.Name,
						Rrtype: mDNS.TypeCNAME,
						Class:  mDNS.ClassINET,
						Ttl:    C.DefaultDNSTTL,
					},
					Target: cname,
				},
			},
		}, nil
	} else if question.Qtype == mDNS.TypeTXT {
		records, err := f.resolver.LookupTXT(ctx, domain)
		if err != nil {
			var dnsError *net.DNSError
			if errors.As(err, &dnsError) && dnsError.IsNotFound {
				return nil, dns.RcodeRefused
			}
			return nil, err
		}
		return &mDNS.Msg{
			MsgHdr: mDNS.MsgHdr{
				Id:       message.Id,
				Rcode:    mDNS.RcodeSuccess,
				Response: true,
			},
			Question: []mDNS.Question{question},
			Answer: []mDNS.RR{
				&mDNS.TXT{
					Hdr: mDNS.RR_Header{
						Name:   question.Name,
						Rrtype: mDNS.TypeCNAME,
						Class:  mDNS.ClassINET,
						Ttl:    C.DefaultDNSTTL,
					},
					Txt: records,
				},
			},
		}, nil
	} else if question.Qtype == mDNS.TypeMX {
		records, err := f.resolver.LookupMX(ctx, domain)
		if err != nil {
			var dnsError *net.DNSError
			if errors.As(err, &dnsError) && dnsError.IsNotFound {
				return nil, dns.RcodeRefused
			}
			return nil, err
		}
		response := &mDNS.Msg{
			MsgHdr: mDNS.MsgHdr{
				Id:       message.Id,
				Rcode:    mDNS.RcodeSuccess,
				Response: true,
			},
			Question: []mDNS.Question{question},
		}
		for _, record := range records {
			response.Answer = append(response.Answer, &mDNS.MX{
				Hdr: mDNS.RR_Header{
					Name:   question.Name,
					Rrtype: mDNS.TypeA,
					Class:  mDNS.ClassINET,
					Ttl:    C.DefaultDNSTTL,
				},
				Preference: record.Pref,
				Mx:         record.Host,
			})
		}
		return response, nil
	} else {
		return nil, E.New("only A, AAAA, NS, CNAME, TXT, MX queries are supported on current platform when using TUN, please switch to a fixed DNS server.")
	}
}
