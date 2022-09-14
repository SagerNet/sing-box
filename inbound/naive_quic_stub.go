//go:build !with_quic

package inbound

import (
	I "github.com/sagernet/sing-box/include"
)

func (n *Naive) configureHTTP3Listener() error {
	return I.ErrQUICNotIncluded
}
