package urltest

import (
	"context"
	"crypto/tls"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
	"github.com/sagernet/sing/common/observable"
)

type HistoryStorage struct {
	access       sync.RWMutex
	delayHistory map[string]*adapter.URLTestHistory
	updateHooks  []*observable.Subscriber[struct{}]
}

func NewHistoryStorage() *HistoryStorage {
	return &HistoryStorage{
		delayHistory: make(map[string]*adapter.URLTestHistory),
	}
}

func (s *HistoryStorage) AddUpdateHook(hook *observable.Subscriber[struct{}]) {
	s.access.Lock()
	defer s.access.Unlock()
	s.updateHooks = append(s.updateHooks, hook)
}

func (s *HistoryStorage) NotifyUpdated() {
	s.access.RLock()
	defer s.access.RUnlock()
	s.notifyUpdated()
}

func (s *HistoryStorage) LoadURLTestHistory(tag string) *adapter.URLTestHistory {
	if s == nil {
		return nil
	}
	s.access.RLock()
	defer s.access.RUnlock()
	return s.delayHistory[tag]
}

func (s *HistoryStorage) DeleteURLTestHistory(tag string) {
	s.access.Lock()
	delete(s.delayHistory, tag)
	s.notifyUpdated()
	s.access.Unlock()
}

func (s *HistoryStorage) StoreURLTestHistory(tag string, history *adapter.URLTestHistory) {
	s.access.Lock()
	s.delayHistory[tag] = history
	s.notifyUpdated()
	s.access.Unlock()
}

// StoreURLTestDelay records a latency sample, preserving any bandwidth sample already
// stored for the tag. Prefer it over StoreURLTestHistory when recording a latency
// probe, so that a manually triggered delay test does not discard throughput data.
func (s *HistoryStorage) StoreURLTestDelay(tag string, delay uint16) {
	s.access.Lock()
	history := &adapter.URLTestHistory{
		Time:  time.Now(),
		Delay: delay,
	}
	if previous := s.delayHistory[tag]; previous != nil {
		history.Throughput = previous.Throughput
		history.Bytes = previous.Bytes
	}
	s.delayHistory[tag] = history
	s.notifyUpdated()
	s.access.Unlock()
}

// StoreURLTestBandwidth records a bandwidth sample, preserving the latency sample. It
// does nothing when no latency sample exists, since an outbound without one is not
// selectable and an entry with a zero delay would corrupt latency ranking.
//
// Entries are replaced rather than mutated in place, because readers hold the pointer
// returned by LoadURLTestHistory outside the lock.
func (s *HistoryStorage) StoreURLTestBandwidth(tag string, throughput uint32, bytes uint32) {
	s.access.Lock()
	previous := s.delayHistory[tag]
	if previous == nil {
		s.access.Unlock()
		return
	}
	s.delayHistory[tag] = &adapter.URLTestHistory{
		Time:       previous.Time,
		Delay:      previous.Delay,
		Throughput: throughput,
		Bytes:      bytes,
	}
	s.notifyUpdated()
	s.access.Unlock()
}

func (s *HistoryStorage) notifyUpdated() {
	for _, updateHook := range s.updateHooks {
		updateHook.Emit(struct{}{})
	}
}

func (s *HistoryStorage) Close() error {
	s.access.Lock()
	defer s.access.Unlock()
	s.updateHooks = nil
	return nil
}

func URLTest(ctx context.Context, link string, detour N.Dialer) (uint16, error) {
	multiplexOutbound, isMultiplexOutbound := common.Cast[adapter.OutboundWithMultiplex](detour)
	if isMultiplexOutbound && multiplexOutbound.MultiplexEnabled() {
		_, err := urlTest(ctx, link, detour)
		if err != nil {
			return 0, err
		}
	}
	return urlTest(ctx, link, detour)
}

func urlTest(ctx context.Context, link string, detour N.Dialer) (t uint16, err error) {
	if link == "" {
		link = "https://www.gstatic.com/generate_204"
	}
	linkURL, err := url.Parse(link)
	if err != nil {
		return
	}
	hostname := linkURL.Hostname()
	port := linkURL.Port()
	if port == "" {
		switch linkURL.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}

	start := time.Now()
	instance, err := detour.DialContext(ctx, "tcp", M.ParseSocksaddrHostPortStr(hostname, port))
	if err != nil {
		return
	}
	defer instance.Close()
	if N.NeedHandshakeForWrite(instance) {
		start = time.Now()
	}
	req, err := http.NewRequest(http.MethodHead, link, nil)
	if err != nil {
		return
	}
	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return instance, nil
			},
			TLSClientConfig: &tls.Config{
				Time:    ntp.TimeFuncFromContext(ctx),
				RootCAs: adapter.RootPoolFromContext(ctx),
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: C.TCPTimeout,
	}
	defer client.CloseIdleConnections()
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return
	}
	resp.Body.Close()
	t = uint16(time.Since(start) / time.Millisecond)
	return
}

// bandwidthReadBufferSize bounds the memory a bandwidth probe retains. The body is
// read into this one reused buffer and discarded, so retained memory is independent
// of the byte cap — which matters on the iOS network extension, where the process
// runs under a hard jetsam limit.
const bandwidthReadBufferSize = 32 * 1024

// BandwidthResult is the outcome of a bounded bandwidth probe.
type BandwidthResult struct {
	// Delay is the time to response headers in milliseconds, measured with the same
	// semantics as URLTest.
	Delay uint16
	// Bytes is the number of body bytes read before the cap or the deadline.
	Bytes uint32
	// Duration is the time spent reading those bytes, excluding Delay, so that the
	// rate reflects the data phase rather than connection setup.
	Duration time.Duration
}

// minTransferDuration floors the measured transfer time. A clock with coarse
// resolution — Windows in particular — reports zero for a transfer that completes
// within one tick, and treating that as zero throughput would rank an immeasurably
// fast path as the worst one and drop it out of contention entirely. Flooring instead
// reports bytes/1ms, which understates such a path but keeps its ordering right.
const minTransferDuration = time.Millisecond

// Throughput returns the effective transfer rate in bytes per second.
func (r *BandwidthResult) Throughput() uint32 {
	if r.Bytes == 0 {
		return 0
	}
	duration := max(r.Duration, minTransferDuration)
	throughput := float64(r.Bytes) / duration.Seconds()
	if throughput >= math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(throughput)
}

// BandwidthTest measures both latency and effective throughput over detour by issuing
// a GET against link and reading at most maxBytes of the response body, cancelling as
// soon as the cap is reached rather than draining the remainder.
//
// It is deliberately a ranking signal rather than a benchmark: a cap in the low
// hundreds of KiB is reached while the flow is still in or near slow start, so the
// absolute rate understates a fast path's true capacity. The ratio between a shaped
// and an unshaped path is already large at that scale, which is what selection needs.
func BandwidthTest(ctx context.Context, link string, detour N.Dialer, maxBytes uint32, timeout time.Duration) (*BandwidthResult, error) {
	multiplexOutbound, isMultiplexOutbound := common.Cast[adapter.OutboundWithMultiplex](detour)
	if isMultiplexOutbound && multiplexOutbound.MultiplexEnabled() {
		// Warm up the multiplex session so its establishment cost is excluded, as in
		// URLTest. A HEAD carries no body, so this does not consume the payload.
		_, err := urlTest(ctx, link, detour)
		if err != nil {
			return nil, err
		}
	}
	return bandwidthTest(ctx, link, detour, maxBytes, timeout)
}

func bandwidthTest(ctx context.Context, link string, detour N.Dialer, maxBytes uint32, timeout time.Duration) (*BandwidthResult, error) {
	linkURL, err := url.Parse(link)
	if err != nil {
		return nil, err
	}
	hostname := linkURL.Hostname()
	port := linkURL.Port()
	if port == "" {
		switch linkURL.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	start := time.Now()
	instance, err := detour.DialContext(ctx, "tcp", M.ParseSocksaddrHostPortStr(hostname, port))
	if err != nil {
		return nil, err
	}
	defer instance.Close()
	if N.NeedHandshakeForWrite(instance) {
		start = time.Now()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, err
	}
	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return instance, nil
			},
			TLSClientConfig: &tls.Config{
				Time:    ntp.TimeFuncFromContext(ctx),
				RootCAs: adapter.RootPoolFromContext(ctx),
			},
			// Measure the bytes on the wire, not what the origin chose to compress.
			DisableCompression: true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: timeout,
	}
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// An error page is small and arrives fast, which would score as an excellent
	// path. Rate limiting in particular must not be mistaken for throughput.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, E.New("unexpected status: ", resp.Status)
	}
	result := &BandwidthResult{
		Delay: uint16(time.Since(start) / time.Millisecond),
	}

	buffer := buf.Get(bandwidthReadBufferSize)
	defer buf.Put(buffer)
	transferStart := time.Now()
	var readBytes uint32
	for readBytes < maxBytes {
		chunk := buffer
		if remaining := maxBytes - readBytes; uint32(len(chunk)) > remaining {
			chunk = chunk[:remaining]
		}
		n, readErr := resp.Body.Read(chunk)
		readBytes += uint32(n)
		if readErr != nil {
			if readErr != io.EOF {
				err = readErr
			}
			break
		}
	}
	result.Duration = time.Since(transferStart)
	result.Bytes = readBytes
	// Cancel before closing so the remainder of the body is torn down rather than
	// drained for connection reuse; draining it would defeat the cap.
	cancel()
	resp.Body.Close()

	if readBytes == 0 {
		if err != nil {
			return nil, err
		}
		return nil, E.New("empty response body")
	}
	// A probe that timed out short of the cap still transferred real bytes, and a
	// timeout is itself evidence of a slow path, so the sample is kept.
	return result, nil
}
