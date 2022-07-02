package outbound

import (
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	F "github.com/sagernet/sing/common/format"
)

func New(router adapter.Router, logger log.Logger, index int, options option.Outbound) (adapter.Outbound, error) {
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
	case C.TypeShadowsocks:
		return NewShadowsocks(router, outboundLogger, options.Tag, options.ShadowsocksOptions)
	default:
		panic(F.ToString("unknown outbound type: ", options.Type))
	}
}
