package geph

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/endpoint"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

const defaultExecutable = "geph5-client"

func RegisterEndpoint(registry *endpoint.Registry) {
	endpoint.Register[option.GephEndpointOptions](registry, C.TypeGeph, NewEndpoint)
}

type Endpoint struct {
	endpoint.Adapter
	logger log.ContextLogger
	stack  packetStack
	proc   *gephProcess
	mu     sync.Mutex
}

func NewEndpoint(ctx context.Context, _ adapter.Router, logger log.ContextLogger, tag string, options option.GephEndpointOptions) (adapter.Endpoint, error) {
	if options.ConfigPath == "" {
		return nil, E.New("missing Geph `config_path`")
	}
	executable := options.ExecutablePath
	if executable == "" {
		executable = defaultExecutable
	}
	if err := validateExtraArgs(options.ExtraArgs); err != nil {
		return nil, err
	}
	p := newGephProcess(ctx, executable, options.ConfigPath, options.ExtraArgs, time.Duration(options.StartupTimeout))
	return &Endpoint{
		Adapter: endpoint.NewAdapter(C.TypeGeph, tag, []string{N.NetworkTCP, N.NetworkUDP}, nil),
		logger:  logger,
		proc:    p,
	}, nil
}

func validateExtraArgs(args []string) error {
	for _, arg := range args {
		if arg == "--config" || arg == "--stdio-vpn" || arg == "--vpn-fd" ||
			len(arg) > len("--config=") && arg[:len("--config=")] == "--config=" {
			return E.New("Geph manages ", arg, "; remove it from `extra_args`")
		}
	}
	return nil
}

func (e *Endpoint) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stack != nil {
		return nil
	}
	if err := e.proc.Start(); err != nil {
		return err
	}
	stack, err := newPacketStack(e.proc.incoming, e.proc.sendPacket)
	if err != nil {
		_ = e.proc.Close()
		return E.Cause(err, "create Geph packet stack")
	}
	e.stack = stack
	return nil
}

func (e *Endpoint) Close() error {
	e.mu.Lock()
	stack := e.stack
	e.stack = nil
	e.mu.Unlock()
	if stack != nil {
		_ = stack.Close()
	}
	if e.proc != nil {
		return e.proc.Close()
	}
	return nil
}

func (e *Endpoint) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if destination.IsDomain() || !destination.Addr.IsValid() {
		return nil, E.New("Geph endpoint requires an IP destination")
	}
	e.mu.Lock()
	stack := e.stack
	e.mu.Unlock()
	if stack == nil {
		return nil, E.New("Geph endpoint is not started")
	}
	return stack.DialContext(ctx, network, destination)
}

func (e *Endpoint) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if destination.IsDomain() || !destination.Addr.IsValid() {
		return nil, E.New("Geph endpoint requires an IP destination")
	}
	e.mu.Lock()
	stack := e.stack
	e.mu.Unlock()
	if stack == nil {
		return nil, E.New("Geph endpoint is not started")
	}
	return stack.ListenPacket(ctx, destination)
}

var _ adapter.Endpoint = (*Endpoint)(nil)
