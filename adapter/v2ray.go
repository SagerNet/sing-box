package adapter

import (
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"net/url"

	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

type V2RayExtraOptionsKeyType int

var V2RayExtraOptionsKey V2RayExtraOptionsKeyType = 1

type V2RayExtraOptions struct {
	URL         *url.URL
	QueryParams url.Values
	Headers     http.Header
}

type V2RayServerTransport interface {
	Network() []string
	Serve(listener net.Listener) error
	ServePacket(listener net.PacketConn) error
	Close() error
}

type V2RayServerTransportHandler interface {
	N.TCPConnectionHandler
	E.Handler
}

type V2RayClientTransport interface {
	DialContext(ctx context.Context) (net.Conn, error)
	Close() error
}

func (options *V2RayExtraOptions) Apply(requestURL *url.URL, headers http.Header) (*url.URL, http.Header) {
	copyURL := *requestURL
	copyHeaders := headers.Clone()
	if options.URL != nil {
		copyURL = *options.URL
	}
	if options.QueryParams != nil {
		rQuery := copyURL.Query()
		for key, values := range options.QueryParams {
			for _, value := range values {
				if rQuery.Has(key) {
					rQuery.Add(key, value)
				} else {
					rQuery.Set(key, value)
				}
			}
		}
		copyURL.RawQuery = base64.RawURLEncoding.EncodeToString([]byte(rQuery.Encode()))
	}
	if options.Headers != nil {
		for key, values := range options.Headers {
			for _, value := range values {
				copyHeaders.Add(key, value)
			}
		}
	}
	return &copyURL, copyHeaders
}
