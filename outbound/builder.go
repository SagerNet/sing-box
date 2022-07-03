package outbound

import (
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

func New(router adapter.Router, logger log.Logger, index int, options option.Outbound) (adapter.Outbound, error) {
	if common.IsEmpty(options) {
		return nil, E.New("empty outbound config")
	}
	var tag string
	if options.Tag != "" {
		tag = options.Tag
	} else {
		tag = F.ToString(index)
	}
	outboundLogger := logger.WithPrefix(F.ToString("outbound/", options.Type, "[", tag, "]: "))
	switch options.Type {
	case C.TypeDirect:
		return NewDirect(router, outboundLogger, options.Tag, options.DirectOptions), nil
	case C.TypeBlock:
		return NewBlock(outboundLogger, options.Tag), nil
	case C.TypeSocks:
		return NewSocks(router, outboundLogger, options.Tag, options.SocksOptions)
	case C.TypeHTTP:
		return NewHTTP(router, outboundLogger, options.Tag, options.HTTPOptions), nil
	case C.TypeShadowsocks:
		return NewShadowsocks(router, outboundLogger, options.Tag, options.ShadowsocksOptions)
	default:
		return nil, E.New("unknown outbound type: ", options.Type)
	}
}
