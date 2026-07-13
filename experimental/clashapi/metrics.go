package clashapi

import (
	"net/http"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/trafficcontrol"
	"github.com/sagernet/sing-box/common/urltest"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const metricsNamespace = "singbox"

// metricsCollector implements prometheus.Collector and exposes sing-box traffic,
// connection and outbound health statistics. All values are read live on each
// scrape from the shared traffic manager, outbound manager and URL test history,
// so the collector holds no state of its own and needs no background goroutine.
type metricsCollector struct {
	trafficManager *trafficcontrol.Manager
	urlTestHistory *urltest.HistoryStorage
	outbound       adapter.OutboundManager

	uplinkBytes           *prometheus.Desc
	downlinkBytes         *prometheus.Desc
	activeConnections     *prometheus.Desc
	connectionsByOutbound *prometheus.Desc
	outboundDelay         *prometheus.Desc
}

func newMetricsCollector(trafficManager *trafficcontrol.Manager, urlTestHistory *urltest.HistoryStorage, outbound adapter.OutboundManager) *metricsCollector {
	return &metricsCollector{
		trafficManager: trafficManager,
		urlTestHistory: urlTestHistory,
		outbound:       outbound,
		uplinkBytes: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, "", "uplink_bytes_total"),
			"Total number of bytes uploaded through all connections.",
			nil, nil,
		),
		downlinkBytes: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, "", "downlink_bytes_total"),
			"Total number of bytes downloaded through all connections.",
			nil, nil,
		),
		activeConnections: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, "", "active_connections"),
			"Number of currently active connections.",
			nil, nil,
		),
		connectionsByOutbound: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, "", "active_connections_by_outbound"),
			"Number of currently active connections grouped by outbound tag.",
			[]string{"outbound"}, nil,
		),
		outboundDelay: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, "", "outbound_delay_milliseconds"),
			"Latest URL test delay of an outbound, in milliseconds.",
			[]string{"outbound"}, nil,
		),
	}
}

func (c *metricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.uplinkBytes
	ch <- c.downlinkBytes
	ch <- c.activeConnections
	ch <- c.connectionsByOutbound
	ch <- c.outboundDelay
}

func (c *metricsCollector) Collect(ch chan<- prometheus.Metric) {
	uplinkTotal, downlinkTotal := c.trafficManager.Total()
	ch <- prometheus.MustNewConstMetric(c.uplinkBytes, prometheus.CounterValue, float64(uplinkTotal))
	ch <- prometheus.MustNewConstMetric(c.downlinkBytes, prometheus.CounterValue, float64(downlinkTotal))
	ch <- prometheus.MustNewConstMetric(c.activeConnections, prometheus.GaugeValue, float64(c.trafficManager.ConnectionsLen()))

	connectionsByOutbound := make(map[string]int)
	for _, metadata := range c.trafficManager.Connections() {
		connectionsByOutbound[metadata.Outbound]++
	}
	for outbound, count := range connectionsByOutbound {
		ch <- prometheus.MustNewConstMetric(c.connectionsByOutbound, prometheus.GaugeValue, float64(count), outbound)
	}

	for _, outbound := range c.outbound.Outbounds() {
		history := c.urlTestHistory.LoadURLTestHistory(outbound.Tag())
		if history == nil {
			continue
		}
		ch <- prometheus.MustNewConstMetric(c.outboundDelay, prometheus.GaugeValue, float64(history.Delay), outbound.Tag())
	}
}

// newMetricsHandler builds an HTTP handler serving the Prometheus text format.
// It uses a dedicated registry so sing-box metrics never collide with the
// default global registry, and includes the standard Go runtime and process
// collectors alongside the sing-box collector.
func newMetricsHandler(trafficManager *trafficcontrol.Manager, urlTestHistory *urltest.HistoryStorage, outbound adapter.OutboundManager) http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		newMetricsCollector(trafficManager, urlTestHistory, outbound),
	)
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}
