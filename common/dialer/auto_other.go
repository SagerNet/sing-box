//go:build !linux

package dialer

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/control"
)

func BindToInterface(router adapter.Router) control.Func {
	return nil
}
