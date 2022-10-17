package balancer

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

// HealthPingSettings holds settings for health Checker
type HealthPingSettings struct {
	Destination   string        `json:"destination"`
	Connectivity  string        `json:"connectivity"`
	Interval      time.Duration `json:"interval"`
	SamplingCount int           `json:"sampling"`
	Timeout       time.Duration `json:"timeout"`
}

// HealthCheck is the health checker for balancers
type HealthCheck struct {
	sync.Mutex

	ticker *time.Ticker
	nodes  []*Node
	logger log.Logger

	Settings *HealthPingSettings
	Results  map[string]*HealthCheckRTTS
}

// NewHealthCheck creates a new HealthPing with settings
func NewHealthCheck(outbounds []*Node, logger log.Logger, config *option.HealthCheckSettings) *HealthCheck {
	settings := &HealthPingSettings{}
	if config != nil {
		settings = &HealthPingSettings{
			Connectivity:  strings.TrimSpace(config.Connectivity),
			Destination:   strings.TrimSpace(config.Destination),
			Interval:      time.Duration(config.Interval),
			SamplingCount: int(config.SamplingCount),
			Timeout:       time.Duration(config.Timeout),
		}
	}
	if settings.Destination == "" {
		settings.Destination = "http://www.google.com/gen_204"
	}
	if settings.Interval == 0 {
		settings.Interval = time.Duration(1) * time.Minute
	} else if settings.Interval < 10 {
		logger.Warn("health check interval is too small, 10s is applied")
		settings.Interval = time.Duration(10) * time.Second
	}
	if settings.SamplingCount <= 0 {
		settings.SamplingCount = 10
	}
	if settings.Timeout <= 0 {
		// results are saved after all health pings finish,
		// a larger timeout could possibly makes checks run longer
		settings.Timeout = time.Duration(5) * time.Second
	}
	return &HealthCheck{
		nodes:    outbounds,
		Settings: settings,
		Results:  nil,
		logger:   logger,
	}
}

// Start starts the health check service
func (h *HealthCheck) Start() {
	if h.ticker != nil {
		return
	}
	interval := h.Settings.Interval * time.Duration(h.Settings.SamplingCount)
	ticker := time.NewTicker(interval)
	h.ticker = ticker
	go func() {
		for {
			h.doCheck(interval, h.Settings.SamplingCount)
			_, ok := <-ticker.C
			if !ok {
				break
			}
		}
	}()
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
	handler string
	value   time.Duration
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
			h.Settings.Destination,
			h.Settings.Timeout,
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
						handler: tag,
						value:   delay,
					}
					return
				}
				if !h.checkConnectivity() {
					h.logger.Debug("network is down")
					ch <- &rtt{
						handler: tag,
						value:   0,
					}
					return
				}
				h.logger.Debug(
					E.Cause(
						err,
						fmt.Sprintf("ping %s via %s", h.Settings.Destination, tag),
					),
				)
				ch <- &rtt{
					handler: tag,
					value:   rttFailed,
				}
			})
		}
	}
	for i := 0; i < count; i++ {
		rtt := <-ch
		if rtt.value > 0 {
			// should not put results when network is down
			h.PutResult(rtt.handler, rtt.value)
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
		validity := h.Settings.Interval * time.Duration(h.Settings.SamplingCount) * 2
		r = NewHealthPingResult(h.Settings.SamplingCount, validity)
		h.Results[tag] = r
	}
	r.Put(rtt)
}

// checkConnectivity checks the network connectivity, it returns
// true if network is good or "connectivity check url" not set
func (h *HealthCheck) checkConnectivity() bool {
	if h.Settings.Connectivity == "" {
		return true
	}
	tester := newDirectPingClient(
		h.Settings.Connectivity,
		h.Settings.Timeout,
	)
	if _, err := tester.MeasureDelay(); err != nil {
		return false
	}
	return true
}
