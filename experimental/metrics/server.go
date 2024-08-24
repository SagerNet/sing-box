package metrics

import (
	"net/http"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var _ adapter.MetricService = (*metricServer)(nil)

func init() {
	experimental.RegisterMetricServerConstructor(NewServer)
}

type metricServer struct {
	http *http.Server

	logger log.Logger
	opts   option.MetricOptions

	packetCountersInbound  *prometheus.CounterVec
	packetCountersOutbound *prometheus.CounterVec
}

func NewServer(logger log.Logger, opts option.MetricOptions) (adapter.MetricService, error) {
	r := chi.NewRouter()
	_server := &http.Server{
		Addr:    opts.Listen,
		Handler: r,
	}
	if opts.Path == "" {
		opts.Path = "/metrics"
	}
	r.Get(opts.Path, promhttp.Handler().ServeHTTP)
	server := &metricServer{
		http:   _server,
		logger: logger,
		opts:   opts,
	}
	err := server.registerMetrics()
	return server, err
}

func (s *metricServer) Start() error {
	if !s.opts.Enabled() {
		return nil
	}
	go func() {
		err := s.http.ListenAndServe()
		if err != nil {
			s.logger.Error("metrics api listen error", err)
		} else {
			s.logger.Info("metrics api listening at ", s.http.Addr, s.opts.Path)
		}
	}()
	return nil
}

func (s *metricServer) Close() error {
	if !s.opts.Enabled() {
		return nil
	}
	return common.Close(common.PtrOrNil(s.http))
}
