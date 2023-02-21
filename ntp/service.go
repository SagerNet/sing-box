package ntp

import (
	"context"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
)

const timeLayout = "2006-01-02 15:04:05 -0700"

var _ adapter.TimeService = (*Service)(nil)

type Service struct {
	ctx    context.Context
	cancel context.CancelFunc
	server M.Socksaddr
	dialer N.Dialer
	logger logger.Logger

	ticker      *time.Ticker
	clockOffset time.Duration
}

func NewService(ctx context.Context, router adapter.Router, logger logger.Logger, options option.NTPOptions) *Service {
	ctx, cancel := context.WithCancel(ctx)
	server := options.ServerOptions.Build()
	if server.Port == 0 {
		server.Port = 123
	}
	var interval time.Duration
	if options.Interval > 0 {
		interval = time.Duration(options.Interval) * time.Second
	} else {
		interval = 30 * time.Minute
	}
	return &Service{
		ctx:    ctx,
		cancel: cancel,
		server: server,
		dialer: dialer.New(router, options.DialerOptions),
		logger: logger,
		ticker: time.NewTicker(interval),
	}
}

func (s *Service) Start() error {
	err := s.update()
	if err != nil {
		return E.Cause(err, "initialize time")
	}
	s.logger.Info("updated time: ", s.TimeFunc()().Local().Format(timeLayout))
	go s.loopUpdate()
	return nil
}

func (s *Service) Close() error {
	s.ticker.Stop()
	s.cancel()
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
			s.logger.Debug("updated time: ", s.TimeFunc()().Local().Format(timeLayout))
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
	return nil
}
