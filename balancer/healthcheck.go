package balancer

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

var (
	_ adapter.Service = (*HealthCheck)(nil)
)

// HealthCheck is the health checker for balancers
type HealthCheck struct {
	mutex sync.Mutex

	ticker *time.Ticker
	router adapter.Router
	tags   []string
	logger log.Logger

	options *option.HealthCheckSettings
	results map[string]*result

	close chan struct{}
}

type result struct {
	// tag      string
	networks []string
	*rttStorage
}

// NewHealthCheck creates a new HealthPing with settings
func NewHealthCheck(router adapter.Router, tags []string, logger log.Logger, config *option.HealthCheckSettings) *HealthCheck {
	if config == nil {
		config = &option.HealthCheckSettings{}
	}
	if config.Destination == "" {
		config.Destination = "http://www.gstatic.com/generate_204"
	}
	if config.Interval == 0 {
		config.Interval = option.Duration(time.Minute)
	} else if config.Interval < 10 {
		logger.Warn("health check interval is too small, 10s is applied")
		config.Interval = option.Duration(10 * time.Second)
	}
	if config.SamplingCount <= 0 {
		config.SamplingCount = 10
	}
	if config.Timeout <= 0 {
		// results are saved after all health pings finish,
		// a larger timeout could possibly makes checks run longer
		config.Timeout = option.Duration(5 * time.Second)
	}
	return &HealthCheck{
		router:  router,
		tags:    tags,
		options: config,
		results: make(map[string]*result),
		logger:  logger,
	}
}

// Start starts the health check service, implements adapter.Service
func (h *HealthCheck) Start() error {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if h.ticker != nil {
		return nil
	}
	h.close = make(chan struct{})
	ticker := time.NewTicker(time.Duration(h.options.Interval))
	h.ticker = ticker
	go func() {
		for {
			select {
			case <-h.close:
				return
			case <-ticker.C:
				h.CheckNodes()
			}
		}
	}()
	return nil
}

// Close stops the health check service, implements adapter.Service
func (h *HealthCheck) Close() error {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if h.ticker != nil {
		h.ticker.Stop()
		h.ticker = nil
	}
	close(h.close)
	return nil
}

// Check does a one time instant health check
func (h *HealthCheck) Check() {
	h.mutex.Lock()
	nodes := h.refreshNodes()
	h.mutex.Unlock()
	for _, n := range nodes {
		go h.checkNode(n)
	}
}

type rtt struct {
	tag   string
	value time.Duration
}

// CheckNodes performs checks for all nodes with random delays
func (h *HealthCheck) CheckNodes() {
	h.mutex.Lock()
	nodes := h.refreshNodes()
	h.mutex.Unlock()
	for _, n := range nodes {
		delay := time.Duration(rand.Intn(int(h.options.Interval)))
		time.AfterFunc(delay, func() {
			h.checkNode(n)
		})
	}
}

func (h *HealthCheck) checkNode(detour adapter.Outbound) {
	tag := detour.Tag()
	client := newPingClient(
		detour,
		h.options.Destination,
		time.Duration(h.options.Timeout),
	)
	// h.logger.Debug("checking ", tag)
	delay, err := client.MeasureDelay()
	if err == nil {
		h.PutResult(tag, delay)
		return
	}
	if !h.checkConnectivity() {
		h.logger.Debug("network is down")
		return
	}
	h.logger.Debug(
		E.Cause(
			err,
			fmt.Sprintf("ping %s via %s", h.options.Destination, tag),
		),
	)
	h.PutResult(tag, rttFailed)
}

// PutResult put a ping rtt to results
func (h *HealthCheck) PutResult(tag string, rtt time.Duration) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	r, ok := h.results[tag]
	if !ok {
		// the result may come after the node is removed
		return
	}
	r.Put(rtt)
}

// checkConnectivity checks the network connectivity, it returns
// true if network is good or "connectivity check url" not set
func (h *HealthCheck) checkConnectivity() bool {
	if h.options.Connectivity == "" {
		return true
	}
	tester := newDirectPingClient(
		h.options.Connectivity,
		time.Duration(h.options.Timeout),
	)
	if _, err := tester.MeasureDelay(); err != nil {
		return false
	}
	return true
}
