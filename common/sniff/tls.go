package sniff

import (
	"context"
	"crypto/tls"
	"io"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
)

var (
	errNotClientHello      = E.New("not Client Hello")
	errPossibleClientHello = E.New("may be Client Hello")
)

func TLSClientHello(ctx context.Context, metadata *adapter.InboundContext, reader io.Reader) error {
	var clientHello *tls.ClientHelloInfo
	err := tls.Server(bufio.NewReadOnlyConn(reader), &tls.Config{
		GetConfigForClient: func(argHello *tls.ClientHelloInfo) (*tls.Config, error) {
			clientHello = argHello
			return nil, nil
		},
	}).HandshakeContext(ctx)
	if clientHello != nil {
		metadata.Protocol = C.ProtocolTLS
		metadata.Domain = clientHello.ServerName
		return nil
	}
	if _, ok := err.(tls.RecordHeaderError); ok {
		return errNotClientHello
	}
	return errPossibleClientHello
}
