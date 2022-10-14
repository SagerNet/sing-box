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
	interval := time.Duration(h.options.Interval) * time.Duration(h.options.SamplingCount)
	ticker := time.NewTicker(interval)
	h.ticker = ticker
	// one time instant check
	h.Check()
	go func() {
		for {
			h.doCheck(interval, h.options.SamplingCount)
			_, ok := <-ticker.C
			if !ok {
				break
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
	return nil
}

// Check does a one time health check
func (h *HealthCheck) Check() {
	go h.doCheck(0, 1)
}

type rtt struct {
	tag   string
	value time.Duration
}

// doCheck performs the 'rounds' amount checks in given 'duration'. You should make
// sure all tags are valid for current balancer
func (h *HealthCheck) doCheck(duration time.Duration, rounds int) {
	nodes := h.refreshNodes()
	count := len(nodes) * rounds
	if count == 0 {
		return
	}
	ch := make(chan *rtt, count)
	// rtts := make(map[string][]time.Duration)
	for _, n := range nodes {
		tag, detour := n.Tag(), n
		client := newPingClient(
			detour,
			h.options.Destination,
			time.Duration(h.options.Timeout),
		)
		for i := 0; i < rounds; i++ {
			delay := time.Duration(0)
			if duration > 0 {
				delay = time.Duration(rand.Intn(int(duration)))
			}
			time.AfterFunc(delay, func() {
				// h.logger.Debug("checking ", tag)
				delay, err := client.MeasureDelay()
				if err == nil {
					ch <- &rtt{
						tag:   tag,
						value: delay,
					}
					return
				}
				if !h.checkConnectivity() {
					h.logger.Debug("network is down")
					ch <- &rtt{
						tag:   tag,
						value: 0,
					}
					return
				}
				h.logger.Debug(
					E.Cause(
						err,
						fmt.Sprintf("ping %s via %s", h.options.Destination, tag),
					),
				)
				ch <- &rtt{
					tag:   tag,
					value: rttFailed,
				}
			})
		}
	}
	for i := 0; i < count; i++ {
		rtt := <-ch
		if rtt.value > 0 {
			// h.logger.Debug("ping ", rtt.tag, ":", rtt.value)
			// should not put results when network is down
			h.putResult(rtt.tag, rtt.value)
		}
	}
}

// putResult put a ping rtt to results
func (h *HealthCheck) putResult(tag string, rtt time.Duration) {
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
