package libbox

import (
	"context"
	"crypto/tls"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/sagernet/sing-box/daemon"
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
	authorization := ""
	if options.Secret != "" {
		authorization = "Bearer " + options.Secret
	}
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(transportCredentials),
		grpc.WithChainUnaryInterceptor(daemon.UnaryClientLocaleInterceptor, func(ctx context.Context, method string, request, reply any, connection *grpc.ClientConn, invoker grpc.UnaryInvoker, options ...grpc.CallOption) error {
			if authorization != "" {
				ctx = metadata.AppendToOutgoingContext(ctx, "authorization", authorization)
			}
			return invoker(ctx, method, request, reply, connection, options...)
		}),
		grpc.WithChainStreamInterceptor(daemon.StreamClientLocaleInterceptor, func(ctx context.Context, description *grpc.StreamDesc, connection *grpc.ClientConn, method string, streamer grpc.Streamer, options ...grpc.CallOption) (grpc.ClientStream, error) {
			if authorization != "" {
				ctx = metadata.AppendToOutgoingContext(ctx, "authorization", authorization)
			}
			return streamer(ctx, description, connection, method, options...)
		}),
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
