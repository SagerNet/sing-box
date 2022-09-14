package v2raygrpc

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	ctx         context.Context
	dialer      N.Dialer
	serverAddr  string
	serviceName string
	dialOptions []grpc.DialOption
	conn        *grpc.ClientConn
	connAccess  sync.Mutex
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayGRPCOptions, tlsConfig *tls.Config) adapter.V2RayClientTransport {
	var dialOptions []grpc.DialOption
	if tlsConfig != nil {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	dialOptions = append(dialOptions, grpc.WithConnectParams(grpc.ConnectParams{
		Backoff: backoff.Config{
			BaseDelay:  500 * time.Millisecond,
			Multiplier: 1.5,
			Jitter:     0.2,
			MaxDelay:   19 * time.Second,
		},
		MinConnectTimeout: 5 * time.Second,
	}))
	dialOptions = append(dialOptions, grpc.WithContextDialer(func(ctx context.Context, server string) (net.Conn, error) {
		return dialer.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddr(server))
	}))
	dialOptions = append(dialOptions, grpc.WithReturnConnectionError())
	return &Client{
		ctx:         ctx,
		dialer:      dialer,
		serverAddr:  serverAddr.String(),
		serviceName: options.ServiceName,
		dialOptions: dialOptions,
	}
}

func (c *Client) Close() error {
	return common.Close(
		common.PtrOrNil(c.conn),
	)
}

func (c *Client) connect() (*grpc.ClientConn, error) {
	conn := c.conn
	if conn != nil && conn.GetState() != connectivity.Shutdown {
		return conn, nil
	}
	c.connAccess.Lock()
	defer c.connAccess.Unlock()
	conn = c.conn
	if conn != nil && conn.GetState() != connectivity.Shutdown {
		return conn, nil
	}
	conn, err := grpc.DialContext(c.ctx, c.serverAddr, c.dialOptions...)
	if err != nil {
		return nil, err
	}
	c.conn = conn
	return conn, nil
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	clientConn, err := c.connect()
	if err != nil {
		return nil, err
	}
	client := NewGunServiceClient(clientConn).(GunServiceCustomNameClient)
	ctx, cancel := context.WithCancel(ctx)
	stream, err := client.TunCustomName(ctx, c.serviceName)
	if err != nil {
		cancel()
		return nil, err
	}
	return NewGRPCConn(stream, cancel), nil
}
