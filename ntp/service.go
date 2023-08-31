package ntp

import (
	"context"
	"os"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/settings"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
)

var _ ntp.TimeService = (*Service)(nil)

type Service struct {
	ctx           context.Context
	cancel        common.ContextCancelCauseFunc
	server        M.Socksaddr
	writeToSystem bool
	dialer        N.Dialer
	logger        logger.Logger
	ticker        *time.Ticker
	clockOffset   time.Duration
}

func NewService(ctx context.Context, router adapter.Router, logger logger.Logger, options option.NTPOptions) (*Service, error) {
	ctx, cancel := common.ContextWithCancelCause(ctx)
	server := options.ServerOptions.Build()
	if server.Port == 0 {
		server.Port = 123
	}
	var interval time.Duration
	if options.Interval > 0 {
		interval = time.Duration(options.Interval)
	} else {
		interval = 30 * time.Minute
	}
	outboundDialer, err := dialer.New(router, options.DialerOptions)
	if err != nil {
		return nil, err
	}
	return &Service{
		ctx:           ctx,
		cancel:        cancel,
		server:        server,
		writeToSystem: options.WriteToSystem,
		dialer:        outboundDialer,
		logger:        logger,
		ticker:        time.NewTicker(interval),
	}, nil
}

func (s *Service) Start() error {
	err := s.update()
	if err != nil {
		return E.Cause(err, "initialize time")
	}
	s.logger.Info("updated time: ", s.TimeFunc()().Local().Format(C.TimeLayout))
	go s.loopUpdate()
	return nil
}

func (s *Service) Close() error {
	s.ticker.Stop()
	s.cancel(os.ErrClosed)
	return nil
}

func (s *Service) TimeFunc() func() time.Time {
	return func() time.Time {
		return time.Now().Add(s.clockOffset)
	}
}

func (s *Service) loopUpdate() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.ticker.C:
		}
		err := s.update()
		if err == nil {
			s.logger.Debug("updated time: ", s.TimeFunc()().Local().Format(C.TimeLayout))
		} else {
			s.logger.Warn("update time: ", err)
		}
	}
}

func (s *Service) update() error {
	response, err := ntp.Exchange(s.ctx, s.dialer, s.server)
	if err != nil {
		return err
	}
	s.clockOffset = response.ClockOffset
	if s.writeToSystem {
		writeErr := settings.SetSystemTime(s.TimeFunc()())
		if writeErr != nil {
			s.logger.Warn("write time to system: ", writeErr)
		}
	}
	return nil
}
