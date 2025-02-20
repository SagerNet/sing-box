package transport

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	mDNS "github.com/miekg/dns"
)

var _ adapter.DNSTransport = (*PredefinedTransport)(nil)

func RegisterPredefined(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.PredefinedDNSServerOptions](registry, C.DNSTypePreDefined, NewPredefined)
}

type PredefinedTransport struct {
	dns.TransportAdapter
	responses []*predefinedResponse
}

type predefinedResponse struct {
	questions []mDNS.Question
	answer    *mDNS.Msg
}

func NewPredefined(ctx context.Context, logger log.ContextLogger, tag string, options option.PredefinedDNSServerOptions) (adapter.DNSTransport, error) {
	var responses []*predefinedResponse
	for _, response := range options.Responses {
		questions, msg, err := response.Build()
		if err != nil {
			return nil, err
		}
		responses = append(responses, &predefinedResponse{
			questions: questions,
			answer:    msg,
		})
	}
	if len(responses) == 0 {
		return nil, E.New("empty predefined responses")
	}
	return &PredefinedTransport{
		TransportAdapter: dns.NewTransportAdapter(C.DNSTypePreDefined, tag, nil),
		responses:        responses,
	}, nil
}

func (t *PredefinedTransport) Reset() {
}

func (t *PredefinedTransport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	for _, response := range t.responses {
		for _, question := range response.questions {
			if func() bool {
				if question.Name == "" && question.Qtype == mDNS.TypeNone {
					return true
				} else if question.Name == "" {
					return common.Any(message.Question, func(it mDNS.Question) bool {
						return it.Qtype == question.Qtype
					})
				} else if question.Qtype == mDNS.TypeNone {
					return common.Any(message.Question, func(it mDNS.Question) bool {
						return it.Name == question.Name
					})
				} else {
					return common.Contains(message.Question, question)
				}
			}() {
				copyAnswer := *response.answer
				copyAnswer.Id = message.Id
				copyAnswer.Question = message.Question
				return &copyAnswer, nil
			}
		}
	}
	return nil, dns.RcodeNameError
}
