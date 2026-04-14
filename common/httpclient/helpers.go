package httpclient

import (
	"context"
	stdTLS "crypto/tls"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/sagernet/sing-box/common/tls"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func dialTLS(ctx context.Context, rawDialer N.Dialer, baseTLSConfig tls.Config, destination M.Socksaddr, nextProtos []string, expectProto string) (net.Conn, error) {
	if baseTLSConfig == nil {
		return nil, E.New("TLS transport unavailable")
	}
	tlsConfig := baseTLSConfig.Clone()
	if tlsConfig.ServerName() == "" && destination.IsValid() {
		tlsConfig.SetServerName(destination.AddrString())
	}
	tlsConfig.SetNextProtos(nextProtos)
	conn, err := rawDialer.DialContext(ctx, N.NetworkTCP, destination)
	if err != nil {
		return nil, err
	}
	tlsConn, err := tls.ClientHandshake(ctx, conn, tlsConfig)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if expectProto != "" && tlsConn.ConnectionState().NegotiatedProtocol != expectProto {
		tlsConn.Close()
		return nil, errHTTP2Fallback
	}
	return tlsConn, nil
}

func applyHeaders(request *http.Request, headers http.Header, host string) {
	for header, values := range headers {
		request.Header[header] = append([]string(nil), values...)
	}
	if host != "" {
		request.Host = host
	}
}

func requestRequiresHTTP1(request *http.Request) bool {
	return strings.Contains(strings.ToLower(request.Header.Get("Connection")), "upgrade") &&
		strings.EqualFold(request.Header.Get("Upgrade"), "websocket")
}

func requestReplayable(request *http.Request) bool {
	return request.Body == nil || request.Body == http.NoBody || request.GetBody != nil
}

func cloneRequestForRetry(request *http.Request) *http.Request {
	cloned := request.Clone(request.Context())
	if request.Body != nil && request.Body != http.NoBody && request.GetBody != nil {
		cloned.Body = mustGetBody(request)
	}
	return cloned
}

func mustGetBody(request *http.Request) io.ReadCloser {
	body, err := request.GetBody()
	if err != nil {
		panic(err)
	}
	return body
}

func buildSTDTLSConfig(baseTLSConfig tls.Config, destination M.Socksaddr, nextProtos []string) (*stdTLS.Config, error) {
	if baseTLSConfig == nil {
		return nil, nil
	}
	tlsConfig := baseTLSConfig.Clone()
	if tlsConfig.ServerName() == "" && destination.IsValid() {
		tlsConfig.SetServerName(destination.AddrString())
	}
	tlsConfig.SetNextProtos(nextProtos)
	return tlsConfig.STDConfig()
}
