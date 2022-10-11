package balancer

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

// HealthCheck is the health checker for balancers
type HealthCheck struct {
	sync.Mutex

	ticker *time.Ticker
	nodes  []*Node
	logger log.Logger

	options *option.HealthCheckSettings
	Results map[string]*HealthCheckRTTS
}

// NewHealthCheck creates a new HealthPing with settings
func NewHealthCheck(outbounds []*Node, logger log.Logger, config *option.HealthCheckSettings) *HealthCheck {
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
		nodes:   outbounds,
		options: config,
		Results: nil,
		logger:  logger,
	}
}

// Start starts the health check service
func (h *HealthCheck) Start() error {
	if h.ticker != nil {
		return nil
	}
	interval := time.Duration(h.options.Interval) * time.Duration(h.options.SamplingCount)
	ticker := time.NewTicker(interval)
	h.ticker = ticker
	go func() {
		h.doCheck(0, 1)
		for {
			_, ok := <-ticker.C
			if !ok {
				break
			}
			h.doCheck(interval, h.options.SamplingCount)
		}
	}()
	return nil
}

// Stop stops the health check service
func (h *HealthCheck) Stop() {
	h.ticker.Stop()
	h.ticker = nil
}

// Check does a one time health check
func (h *HealthCheck) Check() error {
	if len(h.nodes) == 0 {
		return nil
	}
	h.doCheck(0, 1)
	return nil
}

type rtt struct {
	tag   string
	value time.Duration
}

// doCheck performs the 'rounds' amount checks in given 'duration'. You should make
// sure all tags are valid for current balancer
func (h *HealthCheck) doCheck(duration time.Duration, rounds int) {
	count := len(h.nodes) * rounds
	if count == 0 {
		return
	}
	ch := make(chan *rtt, count)
	// rtts := make(map[string][]time.Duration)
	for _, node := range h.nodes {
		tag, detour := node.Outbound.Tag(), node.Outbound
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
			h.PutResult(rtt.tag, rtt.value)
		}
	}
}

// PutResult put a ping rtt to results
func (h *HealthCheck) PutResult(tag string, rtt time.Duration) {
	h.Lock()
	defer h.Unlock()
	if h.Results == nil {
		h.Results = make(map[string]*HealthCheckRTTS)
	}
	r, ok := h.Results[tag]
	if !ok {
		// validity is 2 times to sampling period, since the check are
		// distributed in the time line randomly, in extreme cases,
		// previous checks are distributed on the left, and latters
		// on the right
		validity := time.Duration(h.options.Interval) * time.Duration(h.options.SamplingCount) * 2
		r = NewHealthPingResult(h.options.SamplingCount, validity)
		h.Results[tag] = r
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
