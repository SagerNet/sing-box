package dns

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/netip"
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/net/dns/dnsmessage"
)

const dnsMimeType = "application/dns-message"

var _ adapter.DNSTransport = (*HTTPSTransport)(nil)

type HTTPSTransport struct {
	destination string
	transport   *http.Transport
}

func NewHTTPSTransport(dialer N.Dialer, destination string) *HTTPSTransport {
	return &HTTPSTransport{
		destination: destination,
		transport: &http.Transport{
			ForceAttemptHTTP2: true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
		},
	}
}

func (t *HTTPSTransport) Start() error {
	return nil
}

func (t *HTTPSTransport) Close() error {
	t.transport.CloseIdleConnections()
	return nil
}

func (t *HTTPSTransport) Raw() bool {
	return true
}

func (t *HTTPSTransport) Exchange(ctx context.Context, message *dnsmessage.Message) (*dnsmessage.Message, error) {
	message.ID = 0
	_buffer := buf.StackNewSize(1024)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	rawMessage, err := message.AppendPack(buffer.Index(0))
	if err != nil {
		return nil, err
	}
	buffer.Truncate(len(rawMessage))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, t.destination, bytes.NewReader(buffer.Bytes()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("content-type", dnsMimeType)
	request.Header.Set("accept", dnsMimeType)

	client := &http.Client{Transport: t.transport}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	buffer.FullReset()
	_, err = buffer.ReadAllFrom(response.Body)
	if err != nil {
		return nil, err
	}
	var responseMessage dnsmessage.Message
	err = responseMessage.Unpack(buffer.Bytes())
	if err != nil {
		return nil, err
	}
	return &responseMessage, nil
}

func (t *HTTPSTransport) Lookup(ctx context.Context, domain string, strategy C.DomainStrategy) ([]netip.Addr, error) {
	return nil, os.ErrInvalid
}
