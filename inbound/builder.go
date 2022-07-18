package inbound

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

func New(ctx context.Context, router adapter.Router, logger log.ContextLogger, options option.Inbound) (adapter.Inbound, error) {
	if common.IsEmptyByEquals(options) {
		return nil, E.New("empty inbound config")
	}
	switch options.Type {
	case C.TypeTun:
		return NewTun(ctx, router, logger, options.Tag, options.TunOptions)
	case C.TypeRedirect:
		return NewRedirect(ctx, router, logger, options.Tag, options.RedirectOptions), nil
	case C.TypeTProxy:
		return NewTProxy(ctx, router, logger, options.Tag, options.TProxyOptions), nil
	case C.TypeDNS:
		return NewDNS(ctx, router, logger, options.Tag, options.DNSOptions), nil
	case C.TypeDirect:
		return NewDirect(ctx, router, logger, options.Tag, options.DirectOptions), nil
	case C.TypeSocks:
		return NewSocks(ctx, router, logger, options.Tag, options.SocksOptions), nil
	case C.TypeHTTP:
		return NewHTTP(ctx, router, logger, options.Tag, options.HTTPOptions), nil
	case C.TypeMixed:
		return NewMixed(ctx, router, logger, options.Tag, options.MixedOptions), nil
	case C.TypeShadowsocks:
		return NewShadowsocks(ctx, router, logger, options.Tag, options.ShadowsocksOptions)
	case C.TypeVMess:
		return NewVMess(ctx, router, logger, options.Tag, options.VMessOptions)
	default:
		return nil, E.New("unknown inbound type: ", options.Type)
	}
}
