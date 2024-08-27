package experimental

import (
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
)

type MetricServerConstructor = func(logger log.Logger, options option.MetricOptions) (adapter.MetricService, error)

var metricServerConstructor MetricServerConstructor

func RegisterMetricServerConstructor(constructor MetricServerConstructor) {
	metricServerConstructor = constructor
}

func NewMetricServer(logger log.Logger, options option.MetricOptions) (adapter.MetricService, error) {
	if metricServerConstructor == nil {
		return nil, os.ErrInvalid
	}
	return metricServerConstructor(logger, options)
}
