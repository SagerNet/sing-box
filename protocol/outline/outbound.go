// Package outline implements the smart dialer outbound using the outline-sdk package.
// You can find more details here: https://github.com/Jigsaw-Code/outline-sdk/tree/v0.0.18/x/smart
package outline

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/Jigsaw-Code/outline-sdk/transport"
	"github.com/Jigsaw-Code/outline-sdk/x/smart"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
	"gopkg.in/yaml.v3"
)

// Outbound implements the smart dialer outbound from outline sdk
type Outbound struct {
	outbound.Adapter
	logger       logger.ContextLogger
	dialer       transport.StreamDialer
	dialerMutex  *sync.Mutex
	createDialer func() (transport.StreamDialer, error)
}

// RegisterOutbound registers the outline outbound to the registry
func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.OutboundOutlineOptions](registry, C.TypeOutline, NewOutbound)
}

// NewOutbound creates a proxyless outbond that uses the proxyless transport
// for dialing
func NewOutbound(ctx context.Context, router adapter.Router, log log.ContextLogger, tag string, options option.OutboundOutlineOptions) (adapter.Outbound, error) {
	outboundDialer, err := dialer.New(ctx, options.DialerOptions, true)
	if err != nil {
		return nil, err
	}

	if options.TestTimeout == nil {
		timeout := 10 * time.Second
		options.TestTimeout = &timeout
	}

	outboundStreamDialer := &outboundStreamDialer{
		dialer: outboundDialer,
		logger: log,
	}

	strategyFinder := &smart.StrategyFinder{
		TestTimeout: *options.TestTimeout,
		// TODO: define log writer
		LogWriter:    os.Stdout,
		StreamDialer: outboundStreamDialer,
		PacketDialer: outboundStreamDialer,
	}

	yamlOptions, err := yaml.Marshal(options)
	if err != nil {
		return nil, err
	}
	outbound := &Outbound{
		Adapter:     outbound.NewAdapterWithDialerOptions(C.TypeOutline, tag, []string{network.NetworkTCP}, options.DialerOptions),
		logger:      log,
		dialerMutex: &sync.Mutex{},
		// During the dialer creation the strategy finder try to use the stream dialer
		// for resolving the domains. We can't create the smart dialer during the
		// outbound initialization because there wouldn't be a tunnel to communicate.
		// So for fixing this issue, the dialer must be created during the DialContext call.
		createDialer: sync.OnceValues(func() (transport.StreamDialer, error) {
			dialer, err := strategyFinder.NewDialer(ctx, options.Domains, yamlOptions)
			if err != nil {
				return nil, err
			}
			return dialer, nil
		}),
	}

	return outbound, nil
}

// DialContext extracts the metadata domain, add the destination to the context
// and use the proxyless dialer for sending the request
func (o *Outbound) DialContext(ctx context.Context, network string, destination metadata.Socksaddr) (net.Conn, error) {
	o.dialerMutex.Lock()
	if o.dialer == nil {
		dialer, err := o.createDialer()
		if err != nil {
			return nil, err
		}
		o.dialer = dialer
	}
	o.dialerMutex.Unlock()

	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = o.Tag()
	metadata.Destination = destination

	return o.dialer.DialStream(ctx, fmt.Sprintf("%s:%d", metadata.Domain, destination.Port))
}

// ListenPacket isn't implemented
func (o *Outbound) ListenPacket(ctx context.Context, destination metadata.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

// wrapper around sing-box's network.Dialer to implement streamDialer interface to pass to a
// stream dialer as innerSD
type outboundStreamDialer struct {
	dialer network.Dialer
	logger log.ContextLogger
}

func (s *outboundStreamDialer) DialStream(ctx context.Context, addr string) (transport.StreamConn, error) {
	destination := metadata.ParseSocksaddr(addr)
	conn, err := s.dialer.DialContext(ctx, network.NetworkTCP, destination)
	if err != nil {
		return nil, err
	}
	return conn.(*net.TCPConn), nil
}

func (s *outboundStreamDialer) DialPacket(ctx context.Context, addr string) (net.Conn, error) {
	destination := metadata.ParseSocksaddr(addr)
	conn, err := s.dialer.ListenPacket(ctx, destination)
	if err != nil {
		return nil, err
	}
	return conn.(*net.UDPConn), nil
}
