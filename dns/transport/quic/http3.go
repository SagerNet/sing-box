package quic

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
	sHTTP "github.com/sagernet/sing/protocol/http"

	mDNS "github.com/miekg/dns"
)

var _ adapter.DNSTransport = (*HTTP3Transport)(nil)

func RegisterHTTP3Transport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.RemoteHTTPSDNSServerOptions](registry, C.DNSTypeHTTP3, NewHTTP3)
}

type HTTP3Transport struct {
	dns.TransportAdapter
	logger      logger.ContextLogger
	dialer      N.Dialer
	destination *url.URL
	headers     http.Header
	transport   *http3.Transport
}

func NewHTTP3(ctx context.Context, logger log.ContextLogger, tag string, options option.RemoteHTTPSDNSServerOptions) (adapter.DNSTransport, error) {
	transportDialer, err := dns.NewRemoteDialer(ctx, options.RemoteDNSServerOptions)
	if err != nil {
		return nil, err
	}
	tlsOptions := common.PtrValueOrDefault(options.TLS)
	tlsOptions.Enabled = true
	tlsConfig, err := tls.NewClient(ctx, logger, options.Server, tlsOptions)
	if err != nil {
		return nil, err
	}
	stdConfig, err := tlsConfig.STDConfig()
	if err != nil {
		return nil, err
	}
	headers := options.Headers.Build()
	host := headers.Get("Host")
	if host != "" {
		headers.Del("Host")
	} else {
		if tlsConfig.ServerName() != "" {
			host = tlsConfig.ServerName()
		} else {
			host = options.Server
		}
	}
	destinationURL := url.URL{
		Scheme: "https",
		Host:   host,
	}
	if destinationURL.Host == "" {
		destinationURL.Host = options.Server
	}
	if options.ServerPort != 0 && options.ServerPort != 443 {
		destinationURL.Host = net.JoinHostPort(destinationURL.Host, strconv.Itoa(int(options.ServerPort)))
	}
	path := options.Path
	if path == "" {
		path = "/dns-query"
	}
	err = sHTTP.URLSetPath(&destinationURL, path)
	if err != nil {
		return nil, err
	}
	serverAddr := options.DNSServerAddressOptions.Build()
	if serverAddr.Port == 0 {
		serverAddr.Port = 443
	}
	if !serverAddr.IsValid() {
		return nil, E.New("invalid server address: ", serverAddr)
	}
	return &HTTP3Transport{
		TransportAdapter: dns.NewTransportAdapterWithRemoteOptions(C.DNSTypeHTTP3, tag, options.RemoteDNSServerOptions),
		logger:           logger,
		dialer:           transportDialer,
		destination:      &destinationURL,
		headers:          headers,
		transport: &http3.Transport{
			Dial: func(ctx context.Context, addr string, tlsCfg *tls.STDConfig, cfg *quic.Config) (*quic.Conn, error) {
				conn, dialErr := transportDialer.DialContext(ctx, N.NetworkUDP, serverAddr)
				if dialErr != nil {
					return nil, dialErr
				}
				return quic.DialEarly(ctx, bufio.NewUnbindPacketConn(conn), conn.RemoteAddr(), tlsCfg, cfg)
			},
			TLSClientConfig: stdConfig,
		},
	}, nil
}

func (t *HTTP3Transport) Start(stage adapter.StartStage) error {
	return nil
}

func (t *HTTP3Transport) Close() error {
	return t.transport.Close()
}

func (t *HTTP3Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	exMessage := *message
	exMessage.Id = 0
	exMessage.Compress = true
	requestBuffer := buf.NewSize(1 + message.Len())
	rawMessage, err := exMessage.PackBuffer(requestBuffer.FreeBytes())
	if err != nil {
		requestBuffer.Release()
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, t.destination.String(), bytes.NewReader(rawMessage))
	if err != nil {
		requestBuffer.Release()
		return nil, err
	}
	request.Header = t.headers.Clone()
	request.Header.Set("Content-Type", transport.MimeType)
	request.Header.Set("Accept", transport.MimeType)
	response, err := t.transport.RoundTrip(request)
	requestBuffer.Release()
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, E.New("unexpected status: ", response.Status)
	}
	var responseMessage mDNS.Msg
	if response.ContentLength > 0 {
		responseBuffer := buf.NewSize(int(response.ContentLength))
		_, err = responseBuffer.ReadFullFrom(response.Body, int(response.ContentLength))
		if err != nil {
			return nil, err
		}
		err = responseMessage.Unpack(responseBuffer.Bytes())
		responseBuffer.Release()
	} else {
		rawMessage, err = io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
		err = responseMessage.Unpack(rawMessage)
	}
	if err != nil {
		return nil, err
	}
	return &responseMessage, nil
}
