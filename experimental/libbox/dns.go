package libbox

import (
	"context"
	"net/netip"
	"strings"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport/local"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"

	mDNS "github.com/miekg/dns"
)

type LocalDNSTransport interface {
	Raw() bool
	Lookup(ctx *ExchangeContext, network string, domain string) error
	Exchange(ctx *ExchangeContext, message []byte) error
}

type platformTransport struct {
	dns.TransportAdapter
	iif               LocalDNSTransport
	preferredResolver *local.PreferredDomainResolver
}

func newPlatformTransport(ctx context.Context, logger log.ContextLogger, iif LocalDNSTransport, tag string, options option.LocalDNSServerOptions) (*platformTransport, error) {
	preferredResolver, err := local.NewPreferredDomainResolver(ctx, logger, options)
	if err != nil {
		return nil, err
	}
	return &platformTransport{
		TransportAdapter:  dns.NewTransportAdapterWithLocalOptions(C.DNSTypeLocal, tag, options),
		iif:               iif,
		preferredResolver: preferredResolver,
	}, nil
}

func (p *platformTransport) Start(stage adapter.StartStage) error {
	p.preferredResolver.Start(stage)
	return nil
}

func (p *platformTransport) Close() error {
	return nil
}

func (p *platformTransport) Reset() {
}

func (p *platformTransport) PreferredDomain(domain string) bool {
	return p.preferredResolver.PreferredDomain(domain)
}

func (p *platformTransport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	localResponse := p.preferredResolver.Lookup(message)
	if localResponse != nil {
		return localResponse, nil
	}
	response := &ExchangeContext{
		context: ctx,
	}
	if p.iif.Raw() {
		messageBytes, err := message.Pack()
		if err != nil {
			return nil, err
		}
		done := make(chan error, 1)
		go func() {
			exchangeErr := p.iif.Exchange(response, messageBytes)
			if exchangeErr == nil {
				exchangeErr = response.error
			}
			done <- exchangeErr
		}()
		select {
		case err = <-done:
			if err != nil {
				return nil, err
			}
			return &response.message, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	} else {
		question := message.Question[0]
		var network string
		switch question.Qtype {
		case mDNS.TypeA:
			network = "ip4"
		case mDNS.TypeAAAA:
			network = "ip6"
		default:
			return nil, E.New("only IP queries are supported by current version of Android")
		}
		done := make(chan error, 1)
		go func() {
			lookupErr := p.iif.Lookup(response, network, question.Name)
			if lookupErr == nil {
				lookupErr = response.error
			}
			done <- lookupErr
		}()
		select {
		case err := <-done:
			if err != nil {
				return nil, err
			}
			return dns.FixedResponse(message.Id, question, response.addresses, C.DefaultDNSTTL), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (p *platformTransport) ExchangeAsync(ctx context.Context, message *mDNS.Msg, callback func(response *mDNS.Msg, err error)) {
	go func() {
		callback(p.Exchange(ctx, message))
	}()
}

type Func interface {
	Invoke() error
}

type ExchangeContext struct {
	context   context.Context
	message   mDNS.Msg
	addresses []netip.Addr
	error     error
}

func (c *ExchangeContext) OnCancel(callback Func) {
	go func() {
		<-c.context.Done()
		callback.Invoke()
	}()
}

func (c *ExchangeContext) Success(result string) {
	c.addresses = common.Map(common.Filter(strings.Split(result, "\n"), func(it string) bool {
		return !common.IsEmpty(it)
	}), func(it string) netip.Addr {
		return M.ParseSocksaddrHostPort(it, 0).Unwrap().Addr
	})
}

func (c *ExchangeContext) RawSuccess(result []byte) {
	err := c.message.Unpack(result)
	if err != nil {
		c.error = E.Cause(err, "parse response")
	}
}

func (c *ExchangeContext) ErrorCode(code int32) {
	c.error = dns.RcodeError(code)
}

func (c *ExchangeContext) ErrnoCode(code int32) {
	c.error = syscall.Errno(code)
}

var (
	_ adapter.DNSTransport                    = (*platformTransport)(nil)
	_ adapter.DNSTransportWithPreferredDomain = (*platformTransport)(nil)
)
