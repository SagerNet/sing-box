package oomkiller

import (
	"time"

	"github.com/sagernet/sing/service"
)

func (s *Service) writeOOMReport(memoryUsage uint64) {
	now := time.Now().Unix()
	for {
		lastReport := s.lastReportTime.Load()
		if now-lastReport < 3600 {
			return
		}
		if s.lastReportTime.CompareAndSwap(lastReport, now) {
			break
		}
	}
	reporter := service.FromContext[OOMReporter](s.ctx)
	if reporter == nil {
		return
	}
	err := reporter.WriteReport(memoryUsage)
	if err != nil {
		s.logger.Warn("failed to write OOM report: ", err)
	} else {
		s.logger.Info("OOM report saved")
	}
}
