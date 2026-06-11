package libbox

import (
	"context"
	"crypto/tls"
	"net"
	"net/url"
	"strings"
	"time"

	E "github.com/sagernet/sing/common/exceptions"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type RemoteConnectionOptions struct {
	URL    string
	Secret string
}

const commandClientRemoteProbeTimeout = 10 * time.Second

type remoteConnection struct {
	target      string
	dialOptions []grpc.DialOption
}

func newRemoteConnection(options *RemoteConnectionOptions) (*remoteConnection, error) {
	if options == nil {
		return nil, E.New("missing remote connection options")
	}
	urlString := options.URL
	if !strings.Contains(urlString, "://") {
		urlString = "http://" + urlString
	}
	serverURL, err := url.Parse(urlString)
	if err != nil {
		return nil, E.Cause(err, "parse server URL")
	}
	host := serverURL.Hostname()
	if host == "" {
		return nil, E.New("missing host in server URL: ", options.URL)
	}
	var (
		transportCredentials credentials.TransportCredentials
		defaultPort          string
	)
	switch serverURL.Scheme {
	case "http":
		transportCredentials = insecure.NewCredentials()
		defaultPort = "80"
	case "https":
		transportCredentials = credentials.NewTLS(&tls.Config{ServerName: host})
		defaultPort = "443"
	default:
		return nil, E.New("unsupported server URL scheme: ", serverURL.Scheme, ", expected http or https")
	}
	port := serverURL.Port()
	if port == "" {
		port = defaultPort
	}
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(transportCredentials),
	}
	if options.Secret != "" {
		authorization := "Bearer " + options.Secret
		dialOptions = append(dialOptions,
			grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
				return invoker(metadata.AppendToOutgoingContext(ctx, "authorization", authorization), method, req, reply, cc, opts...)
			}),
			grpc.WithStreamInterceptor(func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
				return streamer(metadata.AppendToOutgoingContext(ctx, "authorization", authorization), desc, cc, method, opts...)
			}),
		)
	}
	return &remoteConnection{
		target:      net.JoinHostPort(host, port),
		dialOptions: dialOptions,
	}, nil
}

func NewRemoteCommandClient(handler CommandClientHandler, options *CommandClientOptions, remoteOptions *RemoteConnectionOptions) (*CommandClient, error) {
	remote, err := newRemoteConnection(remoteOptions)
	if err != nil {
		return nil, err
	}
	client := NewCommandClient(handler, options)
	client.remote = remote
	return client, nil
}

func NewStandaloneRemoteCommandClient(remoteOptions *RemoteConnectionOptions) (*CommandClient, error) {
	remote, err := newRemoteConnection(remoteOptions)
	if err != nil {
		return nil, err
	}
	return &CommandClient{remote: remote}, nil
}
