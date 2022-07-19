package box

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental"
	"github.com/sagernet/sing-box/inbound"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/outbound"
	"github.com/sagernet/sing-box/route"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

var _ adapter.Service = (*Box)(nil)

type Box struct {
	createdAt   time.Time
	router      adapter.Router
	inbounds    []adapter.Inbound
	outbounds   []adapter.Outbound
	logFactory  log.Factory
	logger      log.ContextLogger
	logFile     *os.File
	clashServer adapter.ClashServer
}

func New(ctx context.Context, options option.Options) (*Box, error) {
	createdAt := time.Now()
	logOptions := common.PtrValueOrDefault(options.Log)

	var needClashAPI bool
	if options.Experimental != nil && options.Experimental.ClashAPI != nil && options.Experimental.ClashAPI.ExternalController != "" {
		needClashAPI = true
	}

	var logFactory log.Factory
	var observableLogFactory log.ObservableFactory
	var logFile *os.File
	if logOptions.Disabled {
		logFactory = log.NewNOPFactory()
	} else {
		var logWriter io.Writer
		switch logOptions.Output {
		case "", "stderr":
			logWriter = os.Stderr
		case "stdout":
			logWriter = os.Stdout
		default:
			var err error
			logFile, err = os.OpenFile(logOptions.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				return nil, err
			}
		}
		logFormatter := log.Formatter{
			BaseTime:         createdAt,
			DisableColors:    logOptions.DisableColor || logFile != nil,
			DisableTimestamp: !logOptions.Timestamp && logFile != nil,
			FullTimestamp:    logOptions.Timestamp,
			TimestampFormat:  "-0700 2006-01-02 15:04:05",
		}
		if needClashAPI {
			observableLogFactory = log.NewObservableFactory(logFormatter, logWriter)
			logFactory = observableLogFactory
		} else {
			logFactory = log.NewFactory(logFormatter, logWriter)
		}
		if logOptions.Level != "" {
			logLevel, err := log.ParseLevel(logOptions.Level)
			if err != nil {
				return nil, E.Cause(err, "parse log level")
			}
			logFactory.SetLevel(logLevel)
		} else {
			logFactory.SetLevel(log.LevelTrace)
		}
	}

	router, err := route.NewRouter(
		ctx,
		logFactory.NewLogger("router"),
		logFactory.NewLogger("dns"),
		common.PtrValueOrDefault(options.Route),
		common.PtrValueOrDefault(options.DNS),
	)
	if err != nil {
		return nil, E.Cause(err, "parse route options")
	}
	inbounds := make([]adapter.Inbound, 0, len(options.Inbounds))
	outbounds := make([]adapter.Outbound, 0, len(options.Outbounds))
	for i, inboundOptions := range options.Inbounds {
		var in adapter.Inbound
		var tag string
		if inboundOptions.Tag != "" {
			tag = inboundOptions.Tag
		} else {
			tag = F.ToString(i)
		}
		in, err = inbound.New(
			ctx,
			router,
			logFactory.NewLogger(F.ToString("inbound/", inboundOptions.Type, "[", tag, "]")),
			inboundOptions,
		)
		if err != nil {
			return nil, E.Cause(err, "parse inbound[", i, "]")
		}
		inbounds = append(inbounds, in)
	}
	for i, outboundOptions := range options.Outbounds {
		var out adapter.Outbound
		var tag string
		if outboundOptions.Tag != "" {
			tag = outboundOptions.Tag
		} else {
			tag = F.ToString(i)
		}
		out, err = outbound.New(
			router,
			logFactory.NewLogger(F.ToString("outbound/", outboundOptions.Type, "[", tag, "]")),
			outboundOptions)
		if err != nil {
			return nil, E.Cause(err, "parse outbound[", i, "]")
		}
		outbounds = append(outbounds, out)
	}
	err = router.Initialize(outbounds, func() adapter.Outbound {
		out, oErr := outbound.New(router, logFactory.NewLogger("outbound/direct"), option.Outbound{Type: "direct", Tag: "default"})
		common.Must(oErr)
		outbounds = append(outbounds, out)
		return out
	})
	if err != nil {
		return nil, err
	}

	var clashServer adapter.ClashServer
	if needClashAPI {
		clashServer, err = experimental.NewClashServer(router, observableLogFactory, common.PtrValueOrDefault(options.Experimental.ClashAPI))
		if err != nil {
			return nil, E.Cause(err, "create clash api server")
		}
		router.SetTrafficController(clashServer)
	}
	return &Box{
		router:      router,
		inbounds:    inbounds,
		outbounds:   outbounds,
		createdAt:   createdAt,
		logFactory:  logFactory,
		logger:      logFactory.NewLogger(""),
		logFile:     logFile,
		clashServer: clashServer,
	}, nil
}

func (s *Box) Start() error {
	err := s.router.Start()
	if err != nil {
		return err
	}
	for i, in := range s.inbounds {
		err = in.Start()
		if err != nil {
			for g := 0; g < i; g++ {
				s.inbounds[g].Close()
			}
			return err
		}
	}
	if s.clashServer != nil {
		err = s.clashServer.Start()
		if err != nil {
			return E.Cause(err, "start clash api server")
		}
	}
	s.logger.Info("sing-box started (", F.Seconds(time.Since(s.createdAt).Seconds()), "s)")
	return nil
}

func (s *Box) Close() error {
	for _, in := range s.inbounds {
		in.Close()
	}
	for _, out := range s.outbounds {
		common.Close(out)
	}
	return common.Close(
		s.router,
		s.clashServer,
		common.PtrOrNil(s.logFile),
	)
}
