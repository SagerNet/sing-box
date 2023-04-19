package sniff

import (
	std_bufio "bufio"
	"context"
	"io"
	"net"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/protocol/http"
)

func HTTPHost(ctx context.Context, reader io.Reader) (*adapter.InboundContext, error) {
	request, err := http.ReadRequest(std_bufio.NewReader(reader))
	if err != nil {
		return nil, err
	}
	domain := request.Host
	host, _, err := net.SplitHostPort(domain)
	if err == nil {
		domain = host
	}
	return &adapter.InboundContext{Protocol: C.ProtocolHTTP, Domain: domain}, nil
}
