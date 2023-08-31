package outbound

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func New(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.Outbound) (adapter.Outbound, error) {
	var metadata *adapter.InboundContext
	if tag != "" {
		ctx, metadata = adapter.AppendContext(ctx)
		metadata.Outbound = tag
	}
	if options.Type == "" {
		return nil, E.New("missing outbound type")
	}
	ctx = ContextWithTag(ctx, tag)
	switch options.Type {
	case C.TypeDirect:
		return NewDirect(router, logger, tag, options.DirectOptions)
	case C.TypeBlock:
		return NewBlock(logger, tag), nil
	case C.TypeDNS:
		return NewDNS(router, tag), nil
	case C.TypeSOCKS:
		return NewSocks(router, logger, tag, options.SocksOptions)
	case C.TypeHTTP:
		return NewHTTP(ctx, router, logger, tag, options.HTTPOptions)
	case C.TypeShadowsocks:
		return NewShadowsocks(ctx, router, logger, tag, options.ShadowsocksOptions)
	case C.TypeVMess:
		return NewVMess(ctx, router, logger, tag, options.VMessOptions)
	case C.TypeTrojan:
		return NewTrojan(ctx, router, logger, tag, options.TrojanOptions)
	case C.TypeWireGuard:
		return NewWireGuard(ctx, router, logger, tag, options.WireGuardOptions)
	case C.TypeHysteria:
		return NewHysteria(ctx, router, logger, tag, options.HysteriaOptions)
	case C.TypeTor:
		return NewTor(ctx, router, logger, tag, options.TorOptions)
	case C.TypeSSH:
		return NewSSH(ctx, router, logger, tag, options.SSHOptions)
	case C.TypeShadowTLS:
		return NewShadowTLS(ctx, router, logger, tag, options.ShadowTLSOptions)
	case C.TypeShadowsocksR:
		return NewShadowsocksR(ctx, router, logger, tag, options.ShadowsocksROptions)
	case C.TypeVLESS:
		return NewVLESS(ctx, router, logger, tag, options.VLESSOptions)
	case C.TypeTUIC:
		return NewTUIC(ctx, router, logger, tag, options.TUICOptions)
	case C.TypeHysteria2:
		return NewHysteria2(ctx, router, logger, tag, options.Hysteria2Options)
	case C.TypeSelector:
		return NewSelector(router, logger, tag, options.SelectorOptions)
	case C.TypeURLTest:
		return NewURLTest(ctx, router, logger, tag, options.URLTestOptions)
	default:
		return nil, E.New("unknown outbound type: ", options.Type)
	}
}
