package mieru

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	mieruclient "github.com/enfein/mieru/v3/apis/client"
	mierucommon "github.com/enfein/mieru/v3/apis/common"
	mierumodel "github.com/enfein/mieru/v3/apis/model"
	mierupb "github.com/enfein/mieru/v3/pkg/appctl/appctlpb"
	"google.golang.org/protobuf/proto"
)

type Outbound struct {
	outbound.Adapter
	dialer N.Dialer
	logger log.ContextLogger
	client mieruclient.Client
}

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register(registry, C.TypeMieru, NewOutbound)
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.MieruOutboundOptions) (adapter.Outbound, error) {
	outboundDialer, err := dialer.NewWithOptions(dialer.Options{
		Context:        ctx,
		Options:        options.DialerOptions,
		RemoteIsDomain: options.ServerIsDomain(),
	})
	if err != nil {
		return nil, err
	}

	config, err := buildMieruClientConfig(options, mieruDialer{dialer: outboundDialer})
	if err != nil {
		return nil, fmt.Errorf("failed to build mieru client config: %w", err)
	}
	c := mieruclient.NewClient()
	if err := c.Store(config); err != nil {
		return nil, fmt.Errorf("failed to store mieru client config: %w", err)
	}
	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mieru client: %w", err)
	}
	logger.InfoContext(ctx, "mieru client is started")

	return &Outbound{
		Adapter: outbound.NewAdapterWithDialerOptions(C.TypeMieru, tag, []string{N.NetworkTCP, N.NetworkUDP}, options.DialerOptions),
		dialer:  outboundDialer,
		logger:  logger,
		client:  c,
	}, nil
}

func (o *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = o.Tag()
	metadata.Destination = destination
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		o.logger.InfoContext(ctx, "outbound connection to ", destination)
		d, err := socksAddrToNetAddrSpec(destination, "tcp")
		if err != nil {
			return nil, E.Cause(err, "failed to convert destination address")
		}
		return o.client.DialContext(ctx, d)
	case N.NetworkUDP:
		o.logger.InfoContext(ctx, "outbound UoT packet connection to ", destination)
		d, err := socksAddrToNetAddrSpec(destination, "udp")
		if err != nil {
			return nil, E.Cause(err, "failed to convert destination address")
		}
		streamConn, err := o.client.DialContext(ctx, d)
		if err != nil {
			return nil, err
		}
		return &streamer{
			PacketConn: mierucommon.NewUDPAssociateWrapper(mierucommon.NewPacketOverStreamTunnel(streamConn)),
			Remote:     destination,
		}, nil
	default:
		return nil, os.ErrInvalid
	}
}

func (o *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = o.Tag()
	metadata.Destination = destination
	o.logger.InfoContext(ctx, "outbound UoT packet connection to ", destination)
	d, err := socksAddrToNetAddrSpec(destination, "udp")
	if err != nil {
		return nil, E.Cause(err, "failed to convert destination address")
	}
	streamConn, err := o.client.DialContext(ctx, d)
	if err != nil {
		return nil, err
	}
	return mierucommon.NewUDPAssociateWrapper(mierucommon.NewPacketOverStreamTunnel(streamConn)), nil
}

func (o *Outbound) Close() error {
	return common.Close(o.client)
}

// mieruDialer is an adapter to mieru dialer interface.
type mieruDialer struct {
	dialer N.Dialer
}

func (md mieruDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	addr := M.ParseSocksaddr(address)
	return md.dialer.DialContext(ctx, network, addr)
}

var _ mierucommon.Dialer = (*mieruDialer)(nil)

// streamer converts a net.PacketConn to a net.Conn.
type streamer struct {
	net.PacketConn
	Remote net.Addr
}

var _ net.Conn = (*streamer)(nil)

func (s *streamer) Read(b []byte) (n int, err error) {
	n, _, err = s.PacketConn.ReadFrom(b)
	return
}

func (s *streamer) Write(b []byte) (n int, err error) {
	return s.WriteTo(b, s.Remote)
}

func (s *streamer) RemoteAddr() net.Addr {
	return s.Remote
}

// socksAddrToNetAddrSpec converts a Socksaddr object to NetAddrSpec, and overrides the network.
func socksAddrToNetAddrSpec(sa M.Socksaddr, network string) (mierumodel.NetAddrSpec, error) {
	var nas mierumodel.NetAddrSpec
	if err := nas.From(sa); err != nil {
		return nas, err
	}
	nas.Net = network
	return nas, nil
}

func buildMieruClientConfig(options option.MieruOutboundOptions, dialer mieruDialer) (*mieruclient.ClientConfig, error) {
	if err := validateMieruOptions(options); err != nil {
		return nil, fmt.Errorf("failed to validate mieru options: %w", err)
	}

	transportProtocol := mierupb.TransportProtocol_TCP.Enum()
	server := &mierupb.ServerEndpoint{}
	if options.ServerPort != 0 {
		server.PortBindings = append(server.PortBindings, &mierupb.PortBinding{
			Port:     proto.Int32(int32(options.ServerPort)),
			Protocol: transportProtocol,
		})
	}
	for _, pr := range options.ServerPortRanges {
		server.PortBindings = append(server.PortBindings, &mierupb.PortBinding{
			PortRange: proto.String(pr),
			Protocol:  transportProtocol,
		})
	}
	if options.ServerIsDomain() {
		server.DomainName = proto.String(options.Server)
	} else {
		server.IpAddress = proto.String(options.Server)
	}
	config := &mieruclient.ClientConfig{
		Profile: &mierupb.ClientProfile{
			ProfileName: proto.String("sing-box"),
			User: &mierupb.User{
				Name:     proto.String(options.UserName),
				Password: proto.String(options.Password),
			},
			Servers: []*mierupb.ServerEndpoint{server},
		},
		Dialer: dialer,
	}
	if multiplexing, ok := mierupb.MultiplexingLevel_value[options.Multiplexing]; ok {
		config.Profile.Multiplexing = &mierupb.MultiplexingConfig{
			Level: mierupb.MultiplexingLevel(multiplexing).Enum(),
		}
	}
	return config, nil
}

func validateMieruOptions(options option.MieruOutboundOptions) error {
	if options.Server == "" {
		return fmt.Errorf("server is empty")
	}
	if options.ServerPort == 0 && len(options.ServerPortRanges) == 0 {
		return fmt.Errorf("either server_port or server_ports must be set")
	}
	for _, pr := range options.ServerPortRanges {
		begin, end, err := beginAndEndPortFromPortRange(pr)
		if err != nil {
			return fmt.Errorf("invalid server_ports format")
		}
		if begin < 1 || begin > 65535 {
			return fmt.Errorf("begin port must be between 1 and 65535")
		}
		if end < 1 || end > 65535 {
			return fmt.Errorf("end port must be between 1 and 65535")
		}
		if begin > end {
			return fmt.Errorf("begin port must be less than or equal to end port")
		}
	}
	if options.Transport != "TCP" {
		return fmt.Errorf("transport must be TCP")
	}
	if options.UserName == "" {
		return fmt.Errorf("username is empty")
	}
	if options.Password == "" {
		return fmt.Errorf("password is empty")
	}
	if options.Multiplexing != "" {
		if _, ok := mierupb.MultiplexingLevel_value[options.Multiplexing]; !ok {
			return fmt.Errorf("invalid multiplexing level: %s", options.Multiplexing)
		}
	}
	return nil
}

func beginAndEndPortFromPortRange(portRange string) (int, int, error) {
	var begin, end int
	_, err := fmt.Sscanf(portRange, "%d-%d", &begin, &end)
	return begin, end, err
}
