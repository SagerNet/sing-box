package adapter

import (
	"context"
	"crypto/x509"
	"net"

	N "github.com/sagernet/sing/common/network"
)

type MITMEngine interface {
	Lifecycle
	ExportCertificate() *x509.Certificate
	NewConnection(ctx context.Context, this N.Dialer, conn net.Conn, metadata InboundContext, onClose N.CloseHandlerFunc)
}
