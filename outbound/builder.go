package outbound

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func New(ctx context.Context, router adapter.Router, logger log.ContextLogger, options option.Outbound) (adapter.Outbound, error) {
	if options.Type == "" {
		return nil, E.New("missing outbound type")
	}
	switch options.Type {
	case C.TypeDirect:
		return NewDirect(router, logger, options.Tag, options.DirectOptions), nil
	case C.TypeBlock:
		return NewBlock(logger, options.Tag), nil
	case C.TypeDNS:
		return NewDNS(router, options.Tag), nil
	case C.TypeSocks:
		return NewSocks(router, logger, options.Tag, options.SocksOptions)
	case C.TypeHTTP:
		return NewHTTP(router, logger, options.Tag, options.HTTPOptions)
	case C.TypeShadowsocks:
		return NewShadowsocks(ctx, router, logger, options.Tag, options.ShadowsocksOptions)
	case C.TypeVMess:
		return NewVMess(ctx, router, logger, options.Tag, options.VMessOptions)
	case C.TypeTrojan:
		return NewTrojan(ctx, router, logger, options.Tag, options.TrojanOptions)
	case C.TypeWireGuard:
		return NewWireGuard(ctx, router, logger, options.Tag, options.WireGuardOptions)
	case C.TypeHysteria:
		return NewHysteria(ctx, router, logger, options.Tag, options.HysteriaOptions)
	case C.TypeTor:
		return NewTor(ctx, router, logger, options.Tag, options.TorOptions)
	case C.TypeSSH:
		return NewSSH(ctx, router, logger, options.Tag, options.SSHOptions)
	case C.TypeSelector:
		return NewSelector(router, logger, options.Tag, options.SelectorOptions)
	default:
		return nil, E.New("unknown outbound type: ", options.Type)
	}
}
