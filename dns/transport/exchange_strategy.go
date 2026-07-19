package transport

import (
	"context"
	"sync"

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

// ExchangeRace runs all exchangers concurrently; the first success wins and
// cancels the rest, and when all fail the errors are aggregated.
func ExchangeRace(ctx context.Context, exchangers []AsyncExchanger, callback func(response *mDNS.Msg, err error)) {
	if len(exchangers) == 0 {
		callback(nil, E.New("missing exchangers"))
		return
	}
	if len(exchangers) == 1 {
		exchangers[0](ctx, callback)
		return
	}
	raceCtx, raceCancel := context.WithCancel(ctx)
	state := &raceState{
		cancel:    raceCancel,
		remaining: len(exchangers),
		callback:  callback,
	}
	for _, exchanger := range exchangers {
		exchanger(raceCtx, state.complete)
	}
}

type raceState struct {
	access    sync.Mutex
	done      bool
	remaining int
	errors    []error
	cancel    context.CancelFunc
	callback  func(response *mDNS.Msg, err error)
}

func (s *raceState) complete(response *mDNS.Msg, err error) {
	s.access.Lock()
	if s.done {
		s.access.Unlock()
		return
	}
	if err != nil {
		s.errors = append(s.errors, err)
		if len(s.errors) < s.remaining {
			s.access.Unlock()
			return
		}
		raceErrors := s.errors
		s.done = true
		s.access.Unlock()
		s.cancel()
		s.callback(nil, E.Errors(raceErrors...))
		return
	}
	s.done = true
	s.access.Unlock()
	s.cancel()
	s.callback(response, nil)
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
