package libbox

import (
	"context"
	"net/netip"
	"strings"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/task"

	mDNS "github.com/miekg/dns"
)

type LocalDNSTransport interface {
	Raw() bool
	Lookup(ctx *ExchangeContext, network string, domain string) error
	Exchange(ctx *ExchangeContext, message []byte) error
}

var _ adapter.DNSTransport = (*platformTransport)(nil)

type platformTransport struct {
	dns.TransportAdapter
	iif LocalDNSTransport
}

func newPlatformTransport(iif LocalDNSTransport, tag string, options option.LocalDNSServerOptions) *platformTransport {
	return &platformTransport{
		TransportAdapter: dns.NewTransportAdapterWithLocalOptions(C.DNSTypeLocal, tag, options),
		iif:              iif,
	}
}

func (p *platformTransport) Start(stage adapter.StartStage) error {
	return nil
}

func (p *platformTransport) Close() error {
	return nil
}

func (p *platformTransport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	response := &ExchangeContext{
		context: ctx,
	}
	if p.iif.Raw() {
		messageBytes, err := message.Pack()
		if err != nil {
			return nil, err
		}
		var responseMessage *mDNS.Msg
		var group task.Group
		group.Append0(func(ctx context.Context) error {
			err = p.iif.Exchange(response, messageBytes)
			if err != nil {
				return err
			}
			if response.error != nil {
				return response.error
			}
			responseMessage = &response.message
			return nil
		})
		err = group.Run(ctx)
		if err != nil {
			return nil, err
		}
		return responseMessage, nil
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
		var responseAddrs []netip.Addr
		var group task.Group
		group.Append0(func(ctx context.Context) error {
			err := p.iif.Lookup(response, network, question.Name)
			if err != nil {
				return err
			}
			if response.error != nil {
				return response.error
			}
			responseAddrs = response.addresses
			return nil
		})
		err := group.Run(ctx)
		if err != nil {
			return nil, err
		}
		return dns.FixedResponse(message.Id, question, responseAddrs, C.DefaultDNSTTL), nil
	}
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
