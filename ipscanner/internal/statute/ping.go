package statute

import (
	"context"
	"crypto/tls"
	"fmt"
)

type IPingResult interface {
	Result() IPInfo
	Error() error
	fmt.Stringer
}

type IPing interface {
	Ping() IPingResult
	PingContext(context.Context) IPingResult
}

func TlsVersionToString(ver uint16) string {
	switch ver {
	case tls.VersionSSL30:
		return "SSL 3.0"
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return "unknown"
	}
}
