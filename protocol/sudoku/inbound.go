package sudoku

import (
	"context"
	"net"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	sudokut "github.com/sagernet/sing-box/transport/sudoku"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/sagernet/sing/common/bufio"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.SudokuInboundOptions](registry, C.TypeSudoku, NewInbound)
}

var _ adapter.TCPInjectableInbound = (*Inbound)(nil)

type Inbound struct {
	inbound.Adapter
	ctx       context.Context
	router    adapter.ConnectionRouterEx
	logger    logger.ContextLogger
	listener  *listener.Listener
	protoConf sudokut.ProtocolConfig
	tunnelSrv *sudokut.HTTPMaskTunnelServer
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.SudokuInboundOptions) (adapter.Inbound, error) {
	if options.Key == "" {
		return nil, E.New("missing key")
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

	handshakeTimeout := defaultConf.HandshakeTimeoutSeconds
	if options.HandshakeTimeout > 0 {
		handshakeTimeout = options.HandshakeTimeout
	}

	protoConf := sudokut.ProtocolConfig{
		Key:                     options.Key,
		AEADMethod:              defaultConf.AEADMethod,
		PaddingMin:              paddingMin,
		PaddingMax:              paddingMax,
		EnablePureDownlink:      enablePureDownlink,
		HandshakeTimeoutSeconds: handshakeTimeout,
		DisableHTTPMask:         options.DisableHTTPMask,
		HTTPMaskMode:            defaultConf.HTTPMaskMode,
	}
	if options.AEADMethod != "" {
		protoConf.AEADMethod = options.AEADMethod
	}
	if options.HTTPMaskMode != "" {
		protoConf.HTTPMaskMode = options.HTTPMaskMode
	}

	tables, err := sudokut.NewTablesWithCustomPatterns(protoConf.Key, tableType, options.CustomTable, options.CustomTables)
	if err != nil {
		return nil, E.Cause(err, "build table(s)")
	}
	if len(tables) == 1 {
		protoConf.Table = tables[0]
	} else {
		protoConf.Tables = tables
	}

	in := &Inbound{
		Adapter:   inbound.NewAdapter(C.TypeSudoku, tag),
		ctx:       ctx,
		router:    router,
		logger:    logger,
		protoConf: protoConf,
	}
	in.tunnelSrv = sudokut.NewHTTPMaskTunnelServer(&in.protoConf)
	in.listener = listener.New(listener.Options{
		Context:           ctx,
		Logger:            logger,
		Network:           []string{N.NetworkTCP},
		Listen:            options.ListenOptions,
		ConnectionHandler: in,
	})
	return in, nil
}

func (h *Inbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	return h.listener.Start()
}

func (h *Inbound) Close() error {
	return common.Close(
		h.listener,
		common.PtrOrNil(h.tunnelSrv),
	)
}

func (h *Inbound) NewConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	handshakeConn := conn
	handshakeCfg := &h.protoConf
	if h.tunnelSrv != nil {
		c, cfg, done, err := h.tunnelSrv.WrapConn(conn)
		if err != nil {
			N.CloseOnHandshakeFailure(conn, onClose, err)
			h.logger.ErrorContext(ctx, E.Cause(err, "wrap http tunnel from ", metadata.Source))
			return
		}
		if done {
			return
		}
		if c != nil {
			handshakeConn = c
		}
		if cfg != nil {
			handshakeCfg = cfg
		}
	}

	session, err := sudokut.ServerHandshake(handshakeConn, handshakeCfg)
	if err != nil {
		N.CloseOnHandshakeFailure(handshakeConn, onClose, err)
		if handshakeConn != conn {
			common.Close(conn)
		}
		h.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", metadata.Source))
		return
	}

	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()

	switch session.Type {
	case sudokut.SessionTypeUoT:
		h.logger.InfoContext(ctx, "inbound Sudoku UoT session from ", metadata.Source)
		metadata.Destination = M.Socksaddr{}
		packetConn := bufio.NewPacketConn(sudokut.NewUoTPacketConn(session.Conn))
		h.router.RoutePacketConnectionEx(ctx, packetConn, metadata, onClose)
	default:
		target := M.ParseSocksaddr(session.Target)
		if !target.IsValid() {
			N.CloseOnHandshakeFailure(session.Conn, onClose, E.New("invalid target: ", session.Target))
			return
		}
		metadata.Destination = target
		h.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
		h.router.RouteConnectionEx(ctx, session.Conn, metadata, onClose)
	}
}
