package daemon

import (
	"context"
	"crypto/tls"
	"net"
	"net/url"

	E "github.com/sagernet/sing/common/exceptions"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type RemoteClientOptions struct {
	ServerURL string
	Secret    string
}

func (o RemoteClientOptions) ServerTarget() (string, credentials.TransportCredentials, error) {
	if o.ServerURL == "" {
		return "", nil, E.New("missing server URL")
	}
	serverURL, err := url.Parse(o.ServerURL)
	if err != nil {
		return "", nil, E.Cause(err, "invalid server URL: ", o.ServerURL)
	}
	var enableTLS bool
	switch serverURL.Scheme {
	case "http":
	case "https":
		enableTLS = true
	default:
		return "", nil, E.New("invalid server URL scheme: ", serverURL.Scheme, ", expected http or https")
	}
	host := serverURL.Hostname()
	if host == "" {
		return "", nil, E.New("missing host in server URL: ", o.ServerURL)
	}
	port := serverURL.Port()
	if port == "" {
		if enableTLS {
			port = "443"
		} else {
			port = "80"
		}
	}
	transportCredentials := insecure.NewCredentials()
	if enableTLS {
		transportCredentials = credentials.NewTLS(&tls.Config{ServerName: host})
	}
	return net.JoinHostPort(host, port), transportCredentials, nil
}

func NewRemoteClient(options RemoteClientOptions) (*grpc.ClientConn, error) {
	target, transportCredentials, err := options.ServerTarget()
	if err != nil {
		return nil, err
	}
	return grpc.NewClient(target,
		grpc.WithTransportCredentials(transportCredentials),
		grpc.WithUnaryInterceptor(NewClientAuthUnaryInterceptor(options.Secret)),
		grpc.WithStreamInterceptor(NewClientAuthStreamInterceptor(options.Secret)),
	)
}

func NewClientAuthUnaryInterceptor(secret string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, request, reply any, clientConn *grpc.ClientConn, invoker grpc.UnaryInvoker, options ...grpc.CallOption) error {
		if secret != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+secret)
		}
		return invoker(ctx, method, request, reply, clientConn, options...)
	}
}

func NewClientAuthStreamInterceptor(secret string) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, clientConn *grpc.ClientConn, method string, streamer grpc.Streamer, options ...grpc.CallOption) (grpc.ClientStream, error) {
		if secret != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+secret)
		}
		return streamer(ctx, desc, clientConn, method, options...)
	}
}
