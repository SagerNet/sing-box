//go:build with_quic

package include

import (
	_ "github.com/sagernet/sing-box/transport/v2rayquic"
	_ "github.com/sagernet/sing-dns/quic"
)
