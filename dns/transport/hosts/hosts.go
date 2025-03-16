package hosts

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"

	mDNS "github.com/miekg/dns"
)

func RegisterTransport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.HostsDNSServerOptions](registry, C.DNSTypeHosts, NewTransport)
}

var _ adapter.DNSTransport = (*Transport)(nil)

type Transport struct {
	dns.TransportAdapter
	files []*File
}

func NewTransport(ctx context.Context, logger log.ContextLogger, tag string, options option.HostsDNSServerOptions) (adapter.DNSTransport, error) {
	var files []*File
	if len(options.Path) == 0 {
		files = append(files, NewFile(DefaultPath))
	} else {
		for _, path := range options.Path {
			files = append(files, NewFile(path))
		}
	}
	return &Transport{
		TransportAdapter: dns.NewTransportAdapter(C.DNSTypeHosts, tag, nil),
		files:            files,
	}, nil
}

func (t *Transport) Reset() {
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	question := message.Question[0]
	domain := dns.FqdnToDomain(question.Name)
	if question.Qtype == mDNS.TypeA || question.Qtype == mDNS.TypeAAAA {
		for _, file := range t.files {
			addresses := file.Lookup(domain)
			if len(addresses) > 0 {
				return dns.FixedResponse(message.Id, question, addresses, C.DefaultDNSTTL), nil
			}
		}
	}
	return &mDNS.Msg{
		MsgHdr: mDNS.MsgHdr{
			Id:       message.Id,
			Rcode:    mDNS.RcodeNameError,
			Response: true,
		},
		Question: []mDNS.Question{question},
	}, nil
}
