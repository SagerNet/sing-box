package transport

import (
	"context"
	"strings"
	"sync"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"

	mDNS "github.com/miekg/dns"
)

type AsyncExchanger = func(ctx context.Context, callback func(response *mDNS.Msg, err error))

// ExchangeSequential tries exchangers in order until accept returns true
// (nil accept means err == nil); the last result is delivered as-is.
func ExchangeSequential(ctx context.Context, exchangers []AsyncExchanger, accept func(response *mDNS.Msg, err error) bool, callback func(response *mDNS.Msg, err error)) {
	if len(exchangers) == 0 {
		callback(nil, E.New("missing exchangers"))
		return
	}
	if accept == nil {
		accept = func(response *mDNS.Msg, err error) bool {
			return err == nil
		}
	}
	sequential := &sequentialExchange{
		ctx:        ctx,
		exchangers: exchangers,
		accept:     accept,
		callback:   callback,
	}
	sequential.run(0)
}

type sequentialExchange struct {
	ctx        context.Context
	exchangers []AsyncExchanger
	accept     func(response *mDNS.Msg, err error) bool
	callback   func(response *mDNS.Msg, err error)
}

func (s *sequentialExchange) run(index int) {
	for index < len(s.exchangers) {
		ctxErr := s.ctx.Err()
		if ctxErr != nil {
			s.callback(nil, ctxErr)
			return
		}
		currentIndex := index
		state := &sequentialCallState{}
		s.exchangers[currentIndex](s.ctx, func(response *mDNS.Msg, err error) {
			if currentIndex == len(s.exchangers)-1 || s.accept(response, err) {
				s.callback(response, err)
				return
			}
			state.access.Lock()
			if state.returned {
				state.access.Unlock()
				s.run(currentIndex + 1)
				return
			}
			state.continued = true
			state.access.Unlock()
		})
		state.access.Lock()
		state.returned = true
		continued := state.continued
		state.access.Unlock()
		if !continued {
			return
		}
		index = currentIndex + 1
	}
}

type sequentialCallState struct {
	access    sync.Mutex
	returned  bool
	continued bool
}

func ExchangeParallel(ctx context.Context, exchangers []AsyncExchanger, accept func(response *mDNS.Msg, err error) bool, callback func(response *mDNS.Msg, err error)) {
	if len(exchangers) == 0 {
		callback(nil, E.New("missing exchangers"))
		return
	}
	exchangeCtx, cancel := context.WithCancel(ctx)
	parallel := &parallelExchange{
		accept:    accept,
		callback:  callback,
		cancel:    cancel,
		responses: make([]*mDNS.Msg, len(exchangers)),
		errors:    make([]error, len(exchangers)),
		pending:   len(exchangers),
	}
	for i, exchanger := range exchangers {
		exchanger(exchangeCtx, func(response *mDNS.Msg, err error) {
			parallel.complete(i, response, err)
		})
	}
}

type parallelExchange struct {
	access    sync.Mutex
	accept    func(response *mDNS.Msg, err error) bool
	callback  func(response *mDNS.Msg, err error)
	cancel    context.CancelFunc
	responses []*mDNS.Msg
	errors    []error
	pending   int
	completed bool
}

func (p *parallelExchange) complete(index int, response *mDNS.Msg, err error) {
	p.access.Lock()
	if p.completed {
		p.access.Unlock()
		return
	}
	accepted := p.accept(response, err)
	if !accepted {
		p.responses[index] = response
		p.errors[index] = err
		p.pending--
		if p.pending > 0 {
			p.access.Unlock()
			return
		}
	}
	p.completed = true
	p.access.Unlock()
	p.cancel()
	if accepted {
		p.callback(response, err)
		return
	}
	for i, finalResponse := range p.responses {
		if p.errors[i] == nil {
			p.callback(finalResponse, nil)
			return
		}
	}
	p.callback(nil, E.Errors(p.errors...))
}

func ExchangeNames(ctx context.Context, names []string, question mDNS.Question, exchangerFor func(fqdn string) AsyncExchanger, callback func(response *mDNS.Msg, err error)) {
	if len(names) == 0 {
		callback(nil, E.New("missing name candidates"))
		return
	}
	search := &nameSearchExchange{question: question}
	nameExchangers := common.Map(names, func(fqdn string) AsyncExchanger {
		return search.wrap(fqdn, exchangerFor(fqdn))
	})
	ExchangeSequential(ctx, nameExchangers, func(response *mDNS.Msg, err error) bool {
		return err == nil && response.Rcode != mDNS.RcodeNameError
	}, func(response *mDNS.Msg, err error) {
		if err != nil || response.Rcode == mDNS.RcodeNameError {
			search.access.Lock()
			nameErrorResponse := search.nameErrorResponse
			search.access.Unlock()
			if nameErrorResponse != nil {
				response, err = nameErrorResponse, nil
			}
		}
		callback(response, err)
	})
}

type nameSearchExchange struct {
	question          mDNS.Question
	access            sync.Mutex
	nameErrorResponse *mDNS.Msg
}

func (s *nameSearchExchange) wrap(fqdn string, exchanger AsyncExchanger) AsyncExchanger {
	return func(ctx context.Context, callback func(response *mDNS.Msg, err error)) {
		exchanger(ctx, func(response *mDNS.Msg, err error) {
			if err == nil {
				restoreOriginalQuestion(response, fqdn, s.question)
				if response.Rcode == mDNS.RcodeNameError {
					s.access.Lock()
					if s.nameErrorResponse == nil || fqdn == s.question.Name {
						s.nameErrorResponse = response
					}
					s.access.Unlock()
				}
			}
			callback(response, err)
		})
	}
}

// Stub resolvers discard Answer RRs whose owner name does not match the question.
func restoreOriginalQuestion(response *mDNS.Msg, fqdn string, question mDNS.Question) {
	response.Question = []mDNS.Question{question}
	for _, record := range response.Answer {
		if strings.EqualFold(record.Header().Name, fqdn) {
			record.Header().Name = question.Name
		}
	}
}

func NewFanOutRequest(message *mDNS.Msg, fqdn string, authenticatedData bool) *mDNS.Msg {
	question := message.Question[0]
	question.Name = fqdn
	request := &mDNS.Msg{
		MsgHdr: mDNS.MsgHdr{
			Id:                message.Id,
			RecursionDesired:  true,
			AuthenticatedData: authenticatedData,
		},
		Question: []mDNS.Question{question},
		Compress: true,
	}
	request.SetEdns0(buf.UDPBufferSize, false)
	return request
}
