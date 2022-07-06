package inbound

import (
	"context"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

func New(ctx context.Context, router adapter.Router, logger log.Logger, index int, options option.Inbound) (adapter.Inbound, error) {
	if common.IsEmptyByEquals(options) {
		return nil, E.New("empty inbound config")
	}
	var tag string
	if options.Tag != "" {
		tag = options.Tag
	} else {
		tag = F.ToString(index)
	}
	inboundLogger := logger.WithPrefix(F.ToString("inbound/", options.Type, "[", tag, "]: "))
	switch options.Type {
	case C.TypeDirect:
		return NewDirect(ctx, router, inboundLogger, options.Tag, options.DirectOptions), nil
	case C.TypeSocks:
		return NewSocks(ctx, router, inboundLogger, options.Tag, options.SocksOptions), nil
	case C.TypeHTTP:
		return NewHTTP(ctx, router, inboundLogger, options.Tag, options.HTTPOptions), nil
	case C.TypeMixed:
		return NewMixed(ctx, router, inboundLogger, options.Tag, options.MixedOptions), nil
	case C.TypeShadowsocks:
		return NewShadowsocks(ctx, router, inboundLogger, options.Tag, options.ShadowsocksOptions)
	default:
		return nil, E.New("unknown inbound type: ", options.Type)
	}
}
