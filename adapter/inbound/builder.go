package inbound

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
)

func New(ctx context.Context, router adapter.Router, logger log.Logger, index int, options option.Inbound) (adapter.Inbound, error) {
	var tag string
	if options.Tag != "" {
		tag = options.Tag
	} else {
		tag = F.ToString(index)
	}
	inboundLogger := logger.WithPrefix(F.ToString("inbound/", options.Type, "[", tag, "]: "))
	switch options.Type {
	case C.TypeDirect:
		return NewDirect(ctx, router, inboundLogger, options.Tag, common.PtrValueOrDefault(options.DirectOptions)), nil
	case C.TypeSocks:
		return NewSocks(ctx, router, inboundLogger, options.Tag, common.PtrValueOrDefault(options.SocksOptions)), nil
	case C.TypeHTTP:
		return NewHTTP(ctx, router, inboundLogger, options.Tag, common.PtrValueOrDefault(options.HTTPOptions)), nil
	case C.TypeMixed:
		return NewMixed(ctx, router, inboundLogger, options.Tag, common.PtrValueOrDefault(options.MixedOptions)), nil
	case C.TypeShadowsocks:
		return NewShadowsocks(ctx, router, inboundLogger, options.Tag, common.PtrValueOrDefault(options.ShadowsocksOptions))
	default:
		panic(F.ToString("unknown inbound type: ", options.Type))
	}
}
