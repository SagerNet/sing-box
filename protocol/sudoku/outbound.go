package sudoku

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	sudokut "github.com/sagernet/sing-box/transport/sudoku"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.SudokuOutboundOptions](registry, C.TypeSudoku, NewOutbound)
}

type Outbound struct {
	outbound.Adapter
	ctx            context.Context
	dialer         N.Dialer
	server         M.Socksaddr
	baseConf       sudokut.ProtocolConfig
	httpMaskStrategy string
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.SudokuOutboundOptions) (adapter.Outbound, error) {
	if options.Server == "" {
		return nil, E.New("missing server")
	}
	if options.ServerPort == 0 {
		return nil, E.New("missing server_port")
	}
	if options.Key == "" {
		return nil, E.New("missing key")
	}

	outboundDialer, err := dialer.New(ctx, options.DialerOptions, options.ServerIsDomain())
	if err != nil {
		return nil, err
	}

	defaultConf := sudokut.DefaultConfig()

	tableType := strings.TrimSpace(options.ASCII)
	if tableType == "" {
		tableType = "prefer_ascii"
	}

	paddingMin := defaultConf.PaddingMin
	paddingMax := defaultConf.PaddingMax
	if options.PaddingMin != nil {
		paddingMin = *options.PaddingMin
	}
	if options.PaddingMax != nil {
		paddingMax = *options.PaddingMax
	}
	if options.PaddingMin == nil && options.PaddingMax != nil && paddingMax < paddingMin {
		paddingMin = paddingMax
	}
	if options.PaddingMax == nil && options.PaddingMin != nil && paddingMax < paddingMin {
		paddingMax = paddingMin
	}

	enablePureDownlink := defaultConf.EnablePureDownlink
	if options.EnablePureDownlink != nil {
		enablePureDownlink = *options.EnablePureDownlink
	}

	serverAddress := net.JoinHostPort(options.Server, strconv.Itoa(int(options.ServerPort)))
	baseConf := sudokut.ProtocolConfig{
		ServerAddress:           serverAddress,
		Key:                     options.Key,
		AEADMethod:              defaultConf.AEADMethod,
		PaddingMin:              paddingMin,
		PaddingMax:              paddingMax,
		EnablePureDownlink:      enablePureDownlink,
		HandshakeTimeoutSeconds: defaultConf.HandshakeTimeoutSeconds,
		DisableHTTPMask:         options.DisableHTTPMask,
		HTTPMaskMode:            defaultConf.HTTPMaskMode,
		HTTPMaskTLSEnabled:      options.HTTPMaskTLS,
		HTTPMaskHost:            options.HTTPMaskHost,
	}
	if options.AEADMethod != "" {
		baseConf.AEADMethod = options.AEADMethod
	}
	if options.HTTPMaskMode != "" {
		baseConf.HTTPMaskMode = options.HTTPMaskMode
	}

	tables, err := sudokut.NewTablesWithCustomPatterns(sudokut.ClientAEADSeed(options.Key), tableType, options.CustomTable, options.CustomTables)
	if err != nil {
		return nil, E.Cause(err, "build table(s)")
	}
	if len(tables) == 1 {
		baseConf.Table = tables[0]
	} else {
		baseConf.Tables = tables
	}

	return &Outbound{
		Adapter: outbound.NewAdapterWithDialerOptions(C.TypeSudoku, tag, []string{N.NetworkTCP, N.NetworkUDP}, options.DialerOptions),
		ctx:              ctx,
		dialer:           outboundDialer,
		server:           options.ServerOptions.Build(),
		baseConf:         baseConf,
		httpMaskStrategy: options.HTTPMaskStrategy,
	}, nil
}

func (h *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination

	switch N.NetworkName(network) {
	case N.NetworkTCP:
		metadata.Network = N.NetworkTCP
		return h.dialTCP(ctx, destination)
	case N.NetworkUDP:
		return nil, E.New("UDP dial is not supported, use listen_packet instead")
	default:
		return nil, os.ErrInvalid
	}
}

func (h *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination
	metadata.Network = N.NetworkUDP
	return h.dialUoT(ctx)
}

func (h *Outbound) dialTCP(ctx context.Context, destination M.Socksaddr) (net.Conn, error) {
	cfg := h.baseConf
	cfg.TargetAddress = destination.String()
	if err := cfg.ValidateClient(); err != nil {
		return nil, err
	}

	dialFn := func(dialCtx context.Context, network, addr string) (net.Conn, error) {
		return h.dialer.DialContext(dialCtx, network, M.ParseSocksaddr(addr))
	}

	var (
		rawConn net.Conn
		err     error
	)
	if !cfg.DisableHTTPMask {
		switch strings.ToLower(strings.TrimSpace(cfg.HTTPMaskMode)) {
		case "stream", "poll", "auto":
			rawConn, err = sudokut.DialHTTPMaskTunnel(ctx, cfg.ServerAddress, &cfg, dialFn)
		}
	}
	if rawConn == nil && err == nil {
		rawConn, err = h.dialer.DialContext(ctx, N.NetworkTCP, h.server)
	}
	if err != nil {
		return nil, err
	}

	success := false
	defer func() {
		if !success {
			common.Close(rawConn)
		}
	}()

	handshakeCfg := cfg
	if !handshakeCfg.DisableHTTPMask {
		switch strings.ToLower(strings.TrimSpace(handshakeCfg.HTTPMaskMode)) {
		case "stream", "poll", "auto":
			handshakeCfg.DisableHTTPMask = true
		}
	}
	c, err := sudokut.ClientHandshakeWithOptions(rawConn, &handshakeCfg, sudokut.ClientHandshakeOptions{HTTPMaskStrategy: h.httpMaskStrategy})
	if err != nil {
		return nil, err
	}

	addrBuf, err := sudokut.EncodeAddress(cfg.TargetAddress)
	if err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("encode target address failed: %w", err)
	}
	if _, err := c.Write(addrBuf); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("send target address failed: %w", err)
	}

	success = true
	return c, nil
}

func (h *Outbound) dialUoT(ctx context.Context) (net.PacketConn, error) {
	cfg := h.baseConf
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	dialFn := func(dialCtx context.Context, network, addr string) (net.Conn, error) {
		return h.dialer.DialContext(dialCtx, network, M.ParseSocksaddr(addr))
	}

	var (
		rawConn net.Conn
		err     error
	)
	if !cfg.DisableHTTPMask {
		switch strings.ToLower(strings.TrimSpace(cfg.HTTPMaskMode)) {
		case "stream", "poll", "auto":
			rawConn, err = sudokut.DialHTTPMaskTunnel(ctx, cfg.ServerAddress, &cfg, dialFn)
		}
	}
	if rawConn == nil && err == nil {
		rawConn, err = h.dialer.DialContext(ctx, N.NetworkTCP, h.server)
	}
	if err != nil {
		return nil, err
	}

	success := false
	defer func() {
		if !success {
			common.Close(rawConn)
		}
	}()

	handshakeCfg := cfg
	if !handshakeCfg.DisableHTTPMask {
		switch strings.ToLower(strings.TrimSpace(handshakeCfg.HTTPMaskMode)) {
		case "stream", "poll", "auto":
			handshakeCfg.DisableHTTPMask = true
		}
	}
	c, err := sudokut.ClientHandshakeWithOptions(rawConn, &handshakeCfg, sudokut.ClientHandshakeOptions{HTTPMaskStrategy: h.httpMaskStrategy})
	if err != nil {
		return nil, err
	}

	if err := sudokut.WritePreface(c); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("send uot preface failed: %w", err)
	}

	success = true
	return sudokut.NewUoTPacketConn(c), nil
}

