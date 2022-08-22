//go:build !with_quic

package constant

import E "github.com/sagernet/sing/common/exceptions"

const QUIC_AVAILABLE = false

var ErrQUICNotIncluded = E.New(`QUIC is not included in this build, rebuild with -tags with_quic`)
