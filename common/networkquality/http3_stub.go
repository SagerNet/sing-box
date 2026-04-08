//go:build !with_quic

package networkquality

import (
	C "github.com/sagernet/sing-box/constant"
	N "github.com/sagernet/sing/common/network"
)

func NewHTTP3MeasurementClientFactory(dialer N.Dialer) (MeasurementClientFactory, error) {
	return nil, C.ErrQUICNotIncluded
}
