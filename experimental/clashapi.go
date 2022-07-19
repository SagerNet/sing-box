//go:build with_clash_api

package experimental

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental/clashapi"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

func NewClashServer(router adapter.Router, logFactory log.ObservableFactory, options option.ClashAPIOptions) (adapter.ClashServer, error) {
	return clashapi.NewServer(router, logFactory, options), nil
}
