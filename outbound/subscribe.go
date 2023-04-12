package outbound

import (
	"context"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/subscribe"
	F "github.com/sagernet/sing/common/format"
)

func NewSubscribe(ctx context.Context, router adapter.Router, logFactory log.Factory, tag string, options option.SubscribeOutboundOptions) ([]adapter.Outbound, error) {
	outboundOptions, err := subscribe.ParsePeer(ctx, tag, options)
	if err != nil {
		return nil, err
	}

	outbounds := make([]adapter.Outbound, 0)

	for i, outOptions := range outboundOptions {
		var out adapter.Outbound
		var outTag string
		if outOptions.Tag != "" {
			outTag = outOptions.Tag
		} else {
			outTag = F.ToString(tag, ":", i)
			outOptions.Tag = outTag
		}
		out, err := New(ctx, router, logFactory.NewLogger(F.ToString("outbound/", outOptions.Type, "[", outTag, "]")), outTag, outOptions)
		if err != nil {
			return nil, err
		}
		outbounds = append(outbounds, out)
	}

	return outbounds, nil
}
