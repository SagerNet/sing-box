package script

import (
	"context"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"

	"github.com/adhocore/gronx"
)

var _ adapter.GenericScript = (*SurgeCronScript)(nil)

type SurgeCronScript struct {
	GenericScript
	ctx        context.Context
	expression string
	timer      *time.Timer
}

func NewSurgeCronScript(ctx context.Context, logger logger.ContextLogger, options option.Script) (*SurgeCronScript, error) {
	source, err := NewSource(ctx, logger, options)
	if err != nil {
		return nil, err
	}
	if !gronx.IsValid(options.CronOptions.Expression) {
		return nil, E.New("invalid cron expression: ", options.CronOptions.Expression)
	}
	return &SurgeCronScript{
		GenericScript: GenericScript{
			logger:    logger,
			tag:       options.Tag,
			timeout:   time.Duration(options.Timeout),
			arguments: options.Arguments,
			source:    source,
		},
		ctx:        ctx,
		expression: options.CronOptions.Expression,
	}, nil
}

func (s *SurgeCronScript) Type() string {
	return C.ScriptTypeSurgeCron
}

func (s *SurgeCronScript) Tag() string {
	return s.tag
}

func (s *SurgeCronScript) StartContext(ctx context.Context, startContext *adapter.HTTPStartContext) error {
	return s.source.StartContext(ctx, startContext)
}

func (s *SurgeCronScript) PostStart() error {
	err := s.source.PostStart()
	if err != nil {
		return err
	}
	go s.loop()
	return nil
}

func (s *SurgeCronScript) loop() {
	s.logger.Debug("starting event")
	err := s.Run(s.ctx)
	if err != nil {
		s.logger.Error(E.Cause(err, "running event"))
	}
	nextTick, err := gronx.NextTick(s.expression, false)
	if err != nil {
		s.logger.Error(E.Cause(err, "determine next tick"))
		return
	}
	s.timer = time.NewTimer(nextTick.Sub(time.Now()))
	s.logger.Debug("next event at: ", nextTick.Format(log.DefaultTimeFormat))
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.timer.C:
			s.logger.Debug("starting event")
			err = s.Run(s.ctx)
			if err != nil {
				s.logger.Error(E.Cause(err, "running event"))
			}
			nextTick, err = gronx.NextTick(s.expression, false)
			if err != nil {
				s.logger.Error(E.Cause(err, "determine next tick"))
				return
			}
			s.timer.Reset(nextTick.Sub(time.Now()))
			s.logger.Debug("next event at: ", nextTick)
		}
	}
}

func (s *SurgeCronScript) Close() error {
	return s.source.Close()
}

func (s *SurgeCronScript) Run(ctx context.Context) error {
	program := s.source.Program()
	if program == nil {
		return E.New("invalid script")
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	vm := NewRuntime(ctx, s.logger, cancel)
	err := SetSurgeModules(vm, ctx, s.logger, cancel, s.Tag(), s.Type(), s.arguments)
	if err != nil {
		return err
	}
	return ExecuteSurgeGeneral(vm, program, ctx, s.timeout)
}
