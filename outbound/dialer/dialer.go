package dialer

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	N "github.com/sagernet/sing/common/network"
)

func New(router adapter.Router, options option.DialerOptions) N.Dialer {
	var dialer N.Dialer
	if options.Detour == "" {
		dialer = newDefault(options)
	} else {
		dialer = newDetour(router, options)
	}
	if options.OverrideOptions.IsValid() {
		dialer = newOverride(dialer, common.PtrValueOrDefault(options.OverrideOptions))
	}
	return dialer
}
