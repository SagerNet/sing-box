package constant

import E "github.com/sagernet/sing/common/exceptions"

var ErrTLSRequired = E.New("TLS required")

var ErrQUICNotIncluded = E.New(`QUIC is not included in this build, rebuild with -tags with_quic`)
