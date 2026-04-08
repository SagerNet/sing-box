package networkquality

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	sBufio "github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

const DefaultConfigURL = "https://mensura.cdn-apple.com/api/v1/gm/config"

type Config struct {
	Version      int    `json:"version"`
	TestEndpoint string `json:"test_endpoint"`
	URLs         URLs   `json:"urls"`
}

type URLs struct {
	SmallHTTPSDownloadURL string `json:"small_https_download_url"`
	LargeHTTPSDownloadURL string `json:"large_https_download_url"`
	HTTPSUploadURL        string `json:"https_upload_url"`
	SmallDownloadURL      string `json:"small_download_url"`
	LargeDownloadURL      string `json:"large_download_url"`
	UploadURL             string `json:"upload_url"`
}

func (u *URLs) smallDownloadURL() string {
	if u.SmallHTTPSDownloadURL != "" {
		return u.SmallHTTPSDownloadURL
	}
	return u.SmallDownloadURL
}

func (u *URLs) largeDownloadURL() string {
	if u.LargeHTTPSDownloadURL != "" {
		return u.LargeHTTPSDownloadURL
	}
	return u.LargeDownloadURL
}

func (u *URLs) uploadURL() string {
	if u.HTTPSUploadURL != "" {
		return u.HTTPSUploadURL
	}
	return u.UploadURL
}

type Accuracy int32

const (
	AccuracyLow    Accuracy = 0
	AccuracyMedium Accuracy = 1
	AccuracyHigh   Accuracy = 2
)

func (a Accuracy) String() string {
	switch a {
	case AccuracyHigh:
		return "High"
	case AccuracyMedium:
		return "Medium"
	default:
		return "Low"
	}
}

type Result struct {
	DownloadCapacity         int64
	UploadCapacity           int64
	DownloadRPM              int32
	UploadRPM                int32
	IdleLatencyMs            int32
	DownloadCapacityAccuracy Accuracy
	UploadCapacityAccuracy   Accuracy
	DownloadRPMAccuracy      Accuracy
	UploadRPMAccuracy        Accuracy
}

type Progress struct {
	Phase                    Phase
	DownloadCapacity         int64
	UploadCapacity           int64
	DownloadRPM              int32
	UploadRPM                int32
	IdleLatencyMs            int32
	ElapsedMs                int64
	DownloadCapacityAccuracy Accuracy
	UploadCapacityAccuracy   Accuracy
	DownloadRPMAccuracy      Accuracy
	UploadRPMAccuracy        Accuracy
}

type Phase int32

const (
	PhaseIdle     Phase = 0
	PhaseDownload Phase = 1
	PhaseUpload   Phase = 2
	PhaseDone     Phase = 3
)

type Options struct {
	ConfigURL            string
	HTTPClient           *http.Client
	NewMeasurementClient MeasurementClientFactory
	Serial               bool
	MaxRuntime           time.Duration
	OnProgress           func(Progress)
	Context              context.Context
}

const DefaultMaxRuntime = 20 * time.Second

type measurementSettings struct {
	idleProbeCount      int
	testTimeout         time.Duration
	stabilityInterval   time.Duration
	sampleInterval      time.Duration
	progressInterval    time.Duration
	maxProbesPerSecond  int
	initialConnections  int
	maxConnections      int
	movingAvgDistance   int
	trimPercent         int
	stdDevTolerancePct  float64
	maxProbeCapacityPct float64
}

var settings = measurementSettings{
	idleProbeCount:      5,
	testTimeout:         DefaultMaxRuntime,
	stabilityInterval:   time.Second,
	sampleInterval:      250 * time.Millisecond,
	progressInterval:    500 * time.Millisecond,
	maxProbesPerSecond:  100,
	initialConnections:  1,
	maxConnections:      16,
	movingAvgDistance:   4,
	trimPercent:         5,
	stdDevTolerancePct:  5,
	maxProbeCapacityPct: 0.05,
}

type resolvedConfig struct {
	smallURL        *url.URL
	largeURL        *url.URL
	uploadURL       *url.URL
	connectEndpoint string
}

type directionPlan struct {
	dataURL         *url.URL
	probeURL        *url.URL
	connectEndpoint string
	isUpload        bool
}

type probeTrace struct {
	reused            bool
	connectStart      time.Time
	connectDone       time.Time
	tlsStart          time.Time
	tlsDone           time.Time
	tlsVersion        uint16
	gotConn           time.Time
	wroteRequest      time.Time
	firstResponseByte time.Time
}

type probeMeasurement struct {
	total      time.Duration
	tcp        time.Duration
	tls        time.Duration
	httpFirst  time.Duration
	httpLoaded time.Duration
	bytes      int64
	reused     bool
}

type probeRound struct {
	interval   int
	tcp        time.Duration
	tls        time.Duration
	httpFirst  time.Duration
	httpLoaded time.Duration
}

func (p probeRound) responsivenessLatency() float64 {
	var foreignSamples []float64
	if p.tcp > 0 {
		foreignSamples = append(foreignSamples, durationMillis(p.tcp))
	}
	if p.tls > 0 {
		foreignSamples = append(foreignSamples, durationMillis(p.tls))
	}
	if p.httpFirst > 0 {
		foreignSamples = append(foreignSamples, durationMillis(p.httpFirst))
	}
	if len(foreignSamples) == 0 || p.httpLoaded <= 0 {
		return 0
	}
	return (meanFloat64s(foreignSamples) + durationMillis(p.httpLoaded)) / 2
}

const maxConsecutiveErrors = 3

type loadConnection struct {
	client   *http.Client
	dataURL  *url.URL
	isUpload bool
	active   atomic.Bool
	ready    atomic.Bool
}

func (c *loadConnection) run(ctx context.Context, onError func(error)) {
	defer c.client.CloseIdleConnections()
	markActive := func() {
		c.ready.Store(true)
		c.active.Store(true)
	}
	var consecutiveErrors int
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		var err error
		if c.isUpload {
			err = runUploadRequest(ctx, c.client, c.dataURL.String(), markActive)
		} else {
			err = runDownloadRequest(ctx, c.client, c.dataURL.String(), markActive)
		}
		c.active.Store(false)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			consecutiveErrors++
			if consecutiveErrors > maxConsecutiveErrors {
				onError(err)
				return
			}
			c.client.CloseIdleConnections()
			continue
		}
		consecutiveErrors = 0
	}
}

type intervalThroughput struct {
	interval int
	bps      float64
}

type intervalWindow struct {
	lower int
	upper int
}

type stabilityTracker struct {
	window             int
	stdDevTolerancePct float64
	instantaneous      []float64
	movingAverages     []float64
}

func (s *stabilityTracker) add(value float64) bool {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return false
	}
	s.instantaneous = append(s.instantaneous, value)
	if len(s.instantaneous) > s.window {
		s.instantaneous = s.instantaneous[len(s.instantaneous)-s.window:]
	}
	s.movingAverages = append(s.movingAverages, meanFloat64s(s.instantaneous))
	if len(s.movingAverages) > s.window {
		s.movingAverages = s.movingAverages[len(s.movingAverages)-s.window:]
	}
	return s.stable()
}

func (s *stabilityTracker) ready() bool {
	return len(s.movingAverages) >= s.window
}

func (s *stabilityTracker) accuracy() Accuracy {
	if s.stable() {
		return AccuracyHigh
	}
	if s.ready() {
		return AccuracyMedium
	}
	return AccuracyLow
}

func (s *stabilityTracker) stable() bool {
	if len(s.movingAverages) < s.window {
		return false
	}
	currentAverage := s.movingAverages[len(s.movingAverages)-1]
	if currentAverage <= 0 {
		return false
	}
	return stdDevFloat64s(s.movingAverages) <= currentAverage*(s.stdDevTolerancePct/100)
}

type directionMeasurement struct {
	capacity         int64
	rpm              int32
	capacityAccuracy Accuracy
	rpmAccuracy      Accuracy
}

type directionRunner struct {
	factory    MeasurementClientFactory
	plan       directionPlan
	probeBytes int64

	errCh   chan error
	errOnce sync.Once
	wg      sync.WaitGroup

	totalBytes      atomic.Int64
	currentCapacity atomic.Int64
	currentRPM      atomic.Int32
	currentInterval atomic.Int64

	connMu      sync.Mutex
	connections []*loadConnection

	probeMu              sync.Mutex
	probeRounds          []probeRound
	intervalProbeValues  []float64
	responsivenessWindow *intervalWindow
	throughputs          []intervalThroughput
	throughputWindow     *intervalWindow
}

func newDirectionRunner(factory MeasurementClientFactory, plan directionPlan, probeBytes int64) *directionRunner {
	return &directionRunner{
		factory:    factory,
		plan:       plan,
		probeBytes: probeBytes,
		errCh:      make(chan error, 1),
	}
}

func (r *directionRunner) fail(err error) {
	if err == nil {
		return
	}
	r.errOnce.Do(func() {
		select {
		case r.errCh <- err:
		default:
		}
	})
}

func (r *directionRunner) onConnectionFailed(err error) {
	r.connMu.Lock()
	activeCount := 0
	for _, conn := range r.connections {
		if conn.active.Load() {
			activeCount++
		}
	}
	r.connMu.Unlock()
	if activeCount == 0 {
		r.fail(err)
	}
}

func (r *directionRunner) addConnection(ctx context.Context) error {
	counter := N.CountFunc(func(n int64) { r.totalBytes.Add(n) })
	var readCounters, writeCounters []N.CountFunc
	if r.plan.isUpload {
		writeCounters = []N.CountFunc{counter}
	} else {
		readCounters = []N.CountFunc{counter}
	}
	client, err := r.factory(r.plan.connectEndpoint, true, false, readCounters, writeCounters)
	if err != nil {
		return err
	}
	conn := &loadConnection{
		client:   client,
		dataURL:  r.plan.dataURL,
		isUpload: r.plan.isUpload,
	}
	r.connMu.Lock()
	r.connections = append(r.connections, conn)
	r.connMu.Unlock()
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		conn.run(ctx, r.onConnectionFailed)
	}()
	return nil
}

func (r *directionRunner) connectionCount() int {
	r.connMu.Lock()
	defer r.connMu.Unlock()
	return len(r.connections)
}

func (r *directionRunner) pickReadyConnection() *loadConnection {
	r.connMu.Lock()
	defer r.connMu.Unlock()
	var ready []*loadConnection
	for _, conn := range r.connections {
		if conn.ready.Load() && conn.active.Load() {
			ready = append(ready, conn)
		}
	}
	if len(ready) == 0 {
		return nil
	}
	return ready[rand.Intn(len(ready))]
}

func (r *directionRunner) startProber(ctx context.Context) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(r.probeInterval())
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			conn := r.pickReadyConnection()
			if conn == nil {
				continue
			}
			go func(selfClient *http.Client) {
				foreignClient, err := r.factory(r.plan.connectEndpoint, true, true, nil, nil)
				if err != nil {
					return
				}
				round, err := collectProbeRound(ctx, foreignClient, selfClient, r.plan.probeURL.String())
				foreignClient.CloseIdleConnections()
				if err != nil {
					return
				}
				r.recordProbeRound(probeRound{
					interval:   int(r.currentInterval.Load()),
					tcp:        round.tcp,
					tls:        round.tls,
					httpFirst:  round.httpFirst,
					httpLoaded: round.httpLoaded,
				})
			}(conn.client)
			ticker.Reset(r.probeInterval())
		}
	}()
}

func (r *directionRunner) probeInterval() time.Duration {
	interval := time.Second / time.Duration(settings.maxProbesPerSecond)
	capacity := r.currentCapacity.Load()
	if capacity <= 0 || r.probeBytes <= 0 || settings.maxProbeCapacityPct <= 0 {
		return interval
	}
	bitsPerRound := float64(r.probeBytes*2) * 8
	minSeconds := bitsPerRound / (float64(capacity) * settings.maxProbeCapacityPct)
	if minSeconds <= 0 {
		return interval
	}
	capacityInterval := time.Duration(minSeconds * float64(time.Second))
	if capacityInterval > interval {
		interval = capacityInterval
	}
	return interval
}

func (r *directionRunner) recordProbeRound(round probeRound) {
	r.probeMu.Lock()
	r.probeRounds = append(r.probeRounds, round)
	if latency := round.responsivenessLatency(); latency > 0 {
		r.intervalProbeValues = append(r.intervalProbeValues, latency)
	}
	r.currentRPM.Store(calculateRPM(r.probeRounds))
	r.probeMu.Unlock()
}

func (r *directionRunner) swapIntervalProbeValues() []float64 {
	r.probeMu.Lock()
	defer r.probeMu.Unlock()
	values := append([]float64(nil), r.intervalProbeValues...)
	r.intervalProbeValues = nil
	return values
}

func (r *directionRunner) setResponsivenessWindow(currentInterval int) {
	lower := currentInterval - settings.movingAvgDistance + 1
	if lower < 0 {
		lower = 0
	}
	r.probeMu.Lock()
	r.responsivenessWindow = &intervalWindow{lower: lower, upper: currentInterval}
	r.probeMu.Unlock()
}

func (r *directionRunner) recordThroughput(interval int, bps float64) {
	r.probeMu.Lock()
	r.throughputs = append(r.throughputs, intervalThroughput{interval: interval, bps: bps})
	r.probeMu.Unlock()
}

func (r *directionRunner) setThroughputWindow(currentInterval int) {
	lower := currentInterval - settings.movingAvgDistance + 1
	if lower < 0 {
		lower = 0
	}
	r.probeMu.Lock()
	r.throughputWindow = &intervalWindow{lower: lower, upper: currentInterval}
	r.probeMu.Unlock()
}

func (r *directionRunner) finalRPM() int32 {
	r.probeMu.Lock()
	defer r.probeMu.Unlock()
	if r.responsivenessWindow == nil {
		return calculateRPM(r.probeRounds)
	}
	var rounds []probeRound
	for _, round := range r.probeRounds {
		if round.interval >= r.responsivenessWindow.lower && round.interval <= r.responsivenessWindow.upper {
			rounds = append(rounds, round)
		}
	}
	if len(rounds) == 0 {
		rounds = r.probeRounds
	}
	return calculateRPM(rounds)
}

func (r *directionRunner) finalCapacity(totalDuration time.Duration) int64 {
	r.probeMu.Lock()
	defer r.probeMu.Unlock()
	var samples []float64
	if r.throughputWindow != nil {
		for _, sample := range r.throughputs {
			if sample.interval >= r.throughputWindow.lower && sample.interval <= r.throughputWindow.upper {
				samples = append(samples, sample.bps)
			}
		}
	}
	if len(samples) == 0 {
		for _, sample := range r.throughputs {
			samples = append(samples, sample.bps)
		}
	}
	if len(samples) > 0 {
		return int64(math.Round(upperTrimmedMean(samples, settings.trimPercent)))
	}
	if totalDuration > 0 {
		return int64(float64(r.totalBytes.Load()) * 8 / totalDuration.Seconds())
	}
	return 0
}

func (r *directionRunner) wait() {
	r.wg.Wait()
}

func Run(options Options) (*Result, error) {
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if options.HTTPClient == nil {
		return nil, E.New("http client is required")
	}
	maxRuntime, err := normalizeMaxRuntime(options.MaxRuntime)
	if err != nil {
		return nil, err
	}
	configURL := resolveConfigURL(options.ConfigURL)
	config, err := fetchConfig(ctx, options.HTTPClient, configURL)
	if err != nil {
		return nil, E.Cause(err, "fetch config")
	}
	resolved, err := validateConfig(config)
	if err != nil {
		return nil, E.Cause(err, "validate config")
	}

	start := time.Now()
	report := func(progress Progress) {
		if options.OnProgress == nil {
			return
		}
		progress.ElapsedMs = time.Since(start).Milliseconds()
		options.OnProgress(progress)
	}

	factory := options.NewMeasurementClient
	if factory == nil {
		factory = defaultMeasurementClientFactory(options.HTTPClient)
	}

	report(Progress{Phase: PhaseIdle})
	idleLatency, probeBytes, err := measureIdleLatency(ctx, factory, resolved)
	if err != nil {
		return nil, E.Cause(err, "measure idle latency")
	}
	report(Progress{Phase: PhaseIdle, IdleLatencyMs: idleLatency})

	start = time.Now()

	var download, upload *directionMeasurement
	if options.Serial {
		download, upload, err = measureSerial(
			ctx,
			factory,
			resolved,
			idleLatency,
			probeBytes,
			maxRuntime,
			report,
		)
	} else {
		download, upload, err = measureParallel(
			ctx,
			factory,
			resolved,
			idleLatency,
			probeBytes,
			maxRuntime,
			report,
		)
	}
	if err != nil {
		return nil, err
	}

	result := &Result{
		DownloadCapacity:         download.capacity,
		UploadCapacity:           upload.capacity,
		DownloadRPM:              download.rpm,
		UploadRPM:                upload.rpm,
		IdleLatencyMs:            idleLatency,
		DownloadCapacityAccuracy: download.capacityAccuracy,
		UploadCapacityAccuracy:   upload.capacityAccuracy,
		DownloadRPMAccuracy:      download.rpmAccuracy,
		UploadRPMAccuracy:        upload.rpmAccuracy,
	}
	report(Progress{
		Phase:                    PhaseDone,
		DownloadCapacity:         result.DownloadCapacity,
		UploadCapacity:           result.UploadCapacity,
		DownloadRPM:              result.DownloadRPM,
		UploadRPM:                result.UploadRPM,
		IdleLatencyMs:            result.IdleLatencyMs,
		DownloadCapacityAccuracy: result.DownloadCapacityAccuracy,
		UploadCapacityAccuracy:   result.UploadCapacityAccuracy,
		DownloadRPMAccuracy:      result.DownloadRPMAccuracy,
		UploadRPMAccuracy:        result.UploadRPMAccuracy,
	})
	return result, nil
}

func normalizeMaxRuntime(maxRuntime time.Duration) (time.Duration, error) {
	if maxRuntime == 0 {
		return settings.testTimeout, nil
	}
	if maxRuntime < 0 {
		return 0, E.New("max runtime must be positive")
	}
	return maxRuntime, nil
}

func measureSerial(
	ctx context.Context,
	factory MeasurementClientFactory,
	resolved *resolvedConfig,
	idleLatency int32,
	probeBytes int64,
	maxRuntime time.Duration,
	report func(Progress),
) (*directionMeasurement, *directionMeasurement, error) {
	downloadRuntime, uploadRuntime := splitRuntimeBudget(maxRuntime, 2)
	report(Progress{Phase: PhaseDownload, IdleLatencyMs: idleLatency})
	download, err := measureDirection(ctx, factory, directionPlan{
		dataURL:         resolved.largeURL,
		probeURL:        resolved.smallURL,
		connectEndpoint: resolved.connectEndpoint,
	}, probeBytes, downloadRuntime, func(capacity int64, rpm int32) {
		report(Progress{
			Phase:            PhaseDownload,
			DownloadCapacity: capacity,
			DownloadRPM:      rpm,
			IdleLatencyMs:    idleLatency,
		})
	})
	if err != nil {
		return nil, nil, E.Cause(err, "measure download")
	}

	report(Progress{
		Phase:            PhaseUpload,
		DownloadCapacity: download.capacity,
		DownloadRPM:      download.rpm,
		IdleLatencyMs:    idleLatency,
	})
	upload, err := measureDirection(ctx, factory, directionPlan{
		dataURL:         resolved.uploadURL,
		probeURL:        resolved.smallURL,
		connectEndpoint: resolved.connectEndpoint,
		isUpload:        true,
	}, probeBytes, uploadRuntime, func(capacity int64, rpm int32) {
		report(Progress{
			Phase:            PhaseUpload,
			DownloadCapacity: download.capacity,
			UploadCapacity:   capacity,
			DownloadRPM:      download.rpm,
			UploadRPM:        rpm,
			IdleLatencyMs:    idleLatency,
		})
	})
	if err != nil {
		return nil, nil, E.Cause(err, "measure upload")
	}
	return download, upload, nil
}

func measureParallel(
	ctx context.Context,
	factory MeasurementClientFactory,
	resolved *resolvedConfig,
	idleLatency int32,
	probeBytes int64,
	maxRuntime time.Duration,
	report func(Progress),
) (*directionMeasurement, *directionMeasurement, error) {
	type parallelResult struct {
		measurement *directionMeasurement
		err         error
	}
	type progressState struct {
		sync.Mutex
		downloadCapacity int64
		uploadCapacity   int64
		downloadRPM      int32
		uploadRPM        int32
	}

	parallelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	report(Progress{Phase: PhaseDownload, IdleLatencyMs: idleLatency})
	report(Progress{Phase: PhaseUpload, IdleLatencyMs: idleLatency})

	var state progressState
	sendProgress := func(phase Phase, capacity int64, rpm int32) {
		state.Lock()
		if phase == PhaseDownload {
			state.downloadCapacity = capacity
			state.downloadRPM = rpm
		} else {
			state.uploadCapacity = capacity
			state.uploadRPM = rpm
		}
		snapshot := Progress{
			Phase:            phase,
			DownloadCapacity: state.downloadCapacity,
			UploadCapacity:   state.uploadCapacity,
			DownloadRPM:      state.downloadRPM,
			UploadRPM:        state.uploadRPM,
			IdleLatencyMs:    idleLatency,
		}
		state.Unlock()
		report(snapshot)
	}

	var wg sync.WaitGroup
	downloadCh := make(chan parallelResult, 1)
	uploadCh := make(chan parallelResult, 1)
	wg.Add(2)
	go func() {
		defer wg.Done()
		measurement, err := measureDirection(parallelCtx, factory, directionPlan{
			dataURL:         resolved.largeURL,
			probeURL:        resolved.smallURL,
			connectEndpoint: resolved.connectEndpoint,
		}, probeBytes, maxRuntime, func(capacity int64, rpm int32) {
			sendProgress(PhaseDownload, capacity, rpm)
		})
		if err != nil {
			cancel()
			downloadCh <- parallelResult{err: E.Cause(err, "measure download")}
			return
		}
		downloadCh <- parallelResult{measurement: measurement}
	}()
	go func() {
		defer wg.Done()
		measurement, err := measureDirection(parallelCtx, factory, directionPlan{
			dataURL:         resolved.uploadURL,
			probeURL:        resolved.smallURL,
			connectEndpoint: resolved.connectEndpoint,
			isUpload:        true,
		}, probeBytes, maxRuntime, func(capacity int64, rpm int32) {
			sendProgress(PhaseUpload, capacity, rpm)
		})
		if err != nil {
			cancel()
			uploadCh <- parallelResult{err: E.Cause(err, "measure upload")}
			return
		}
		uploadCh <- parallelResult{measurement: measurement}
	}()

	download := <-downloadCh
	upload := <-uploadCh
	wg.Wait()
	if download.err != nil {
		return nil, nil, download.err
	}
	if upload.err != nil {
		return nil, nil, upload.err
	}
	return download.measurement, upload.measurement, nil
}

func resolveConfigURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return DefaultConfigURL
	}
	if !strings.Contains(rawURL, "://") && !strings.Contains(rawURL, "/") {
		return "https://" + rawURL + "/.well-known/nq"
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if parsedURL.Scheme != "" && parsedURL.Host != "" && (parsedURL.Path == "" || parsedURL.Path == "/") {
		parsedURL.Path = "/.well-known/nq"
		return parsedURL.String()
	}
	return rawURL
}

func fetchConfig(ctx context.Context, client *http.Client, configURL string) (*Config, error) {
	req, err := newRequest(ctx, http.MethodGet, configURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err = validateResponse(resp); err != nil {
		return nil, err
	}
	var config Config
	if err = json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, E.Cause(err, "decode config")
	}
	return &config, nil
}

func validateConfig(config *Config) (*resolvedConfig, error) {
	if config == nil {
		return nil, E.New("config is nil")
	}
	if config.Version != 1 {
		return nil, E.New("unsupported config version: ", config.Version)
	}
	parseURL := func(name string, rawURL string) (*url.URL, error) {
		if rawURL == "" {
			return nil, E.New("config missing required URL: ", name)
		}
		parsedURL, err := url.Parse(rawURL)
		if err != nil {
			return nil, E.Cause(err, "parse "+name)
		}
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return nil, E.New("unsupported URL scheme in ", name, ": ", parsedURL.Scheme)
		}
		if parsedURL.Host == "" {
			return nil, E.New("config missing host in ", name)
		}
		return parsedURL, nil
	}

	smallURL, err := parseURL("small_download_url", config.URLs.smallDownloadURL())
	if err != nil {
		return nil, err
	}
	largeURL, err := parseURL("large_download_url", config.URLs.largeDownloadURL())
	if err != nil {
		return nil, err
	}
	uploadURL, err := parseURL("upload_url", config.URLs.uploadURL())
	if err != nil {
		return nil, err
	}

	if smallURL.Host != largeURL.Host || smallURL.Host != uploadURL.Host {
		return nil, E.New("config URLs must use the same host")
	}

	return &resolvedConfig{
		smallURL:        smallURL,
		largeURL:        largeURL,
		uploadURL:       uploadURL,
		connectEndpoint: strings.TrimSpace(config.TestEndpoint),
	}, nil
}

func measureIdleLatency(ctx context.Context, factory MeasurementClientFactory, config *resolvedConfig) (int32, int64, error) {
	var latencies []int64
	var maxProbeBytes int64
	for i := 0; i < settings.idleProbeCount; i++ {
		select {
		case <-ctx.Done():
			return 0, 0, ctx.Err()
		default:
		}
		client, err := factory(config.connectEndpoint, true, true, nil, nil)
		if err != nil {
			return 0, 0, err
		}
		measurement, err := runProbe(ctx, client, config.smallURL.String(), false)
		client.CloseIdleConnections()
		if err != nil {
			return 0, 0, err
		}
		latencies = append(latencies, measurement.total.Milliseconds())
		if measurement.bytes > maxProbeBytes {
			maxProbeBytes = measurement.bytes
		}
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	return int32(latencies[len(latencies)/2]), maxProbeBytes, nil
}

func measureDirection(
	ctx context.Context,
	factory MeasurementClientFactory,
	plan directionPlan,
	probeBytes int64,
	maxRuntime time.Duration,
	onProgress func(capacity int64, rpm int32),
) (*directionMeasurement, error) {
	phaseCtx, phaseCancel := context.WithTimeout(ctx, maxRuntime)
	defer phaseCancel()

	runner := newDirectionRunner(factory, plan, probeBytes)
	defer runner.wait()

	for i := 0; i < settings.initialConnections; i++ {
		err := runner.addConnection(phaseCtx)
		if err != nil {
			return nil, err
		}
	}

	runner.startProber(phaseCtx)

	throughputTracker := stabilityTracker{
		window:             settings.movingAvgDistance,
		stdDevTolerancePct: settings.stdDevTolerancePct,
	}
	responsivenessTracker := stabilityTracker{
		window:             settings.movingAvgDistance,
		stdDevTolerancePct: settings.stdDevTolerancePct,
	}

	start := time.Now()
	sampleTicker := time.NewTicker(settings.sampleInterval)
	defer sampleTicker.Stop()
	intervalTicker := time.NewTicker(settings.stabilityInterval)
	defer intervalTicker.Stop()
	progressTicker := time.NewTicker(settings.progressInterval)
	defer progressTicker.Stop()

	prevSampleBytes := int64(0)
	prevSampleTime := start
	prevIntervalBytes := int64(0)
	prevIntervalTime := start
	var ewmaCapacity float64
	var goodputSaturated bool
	var intervalIndex int

	for {
		select {
		case err := <-runner.errCh:
			return nil, err
		case now := <-sampleTicker.C:
			currentBytes := runner.totalBytes.Load()
			elapsed := now.Sub(prevSampleTime).Seconds()
			if elapsed > 0 {
				instantaneousBps := float64(currentBytes-prevSampleBytes) * 8 / elapsed
				if ewmaCapacity == 0 {
					ewmaCapacity = instantaneousBps
				} else {
					ewmaCapacity = 0.3*instantaneousBps + 0.7*ewmaCapacity
				}
				runner.currentCapacity.Store(int64(ewmaCapacity))
			}
			prevSampleBytes = currentBytes
			prevSampleTime = now
		case <-intervalTicker.C:
			now := time.Now()
			currentBytes := runner.totalBytes.Load()
			elapsed := now.Sub(prevIntervalTime).Seconds()
			if elapsed > 0 {
				intervalBps := float64(currentBytes-prevIntervalBytes) * 8 / elapsed
				runner.recordThroughput(intervalIndex, intervalBps)
				throughputStable := throughputTracker.add(intervalBps)
				if throughputStable && runner.throughputWindow == nil {
					runner.setThroughputWindow(intervalIndex)
				}
				if !goodputSaturated && (throughputStable || (runner.connectionCount() >= settings.maxConnections && throughputTracker.ready())) {
					goodputSaturated = true
				}
				if runner.connectionCount() < settings.maxConnections {
					err := runner.addConnection(phaseCtx)
					if err != nil {
						return nil, err
					}
				}
			}
			if goodputSaturated {
				if values := runner.swapIntervalProbeValues(); len(values) > 0 {
					if responsivenessTracker.add(upperTrimmedMean(values, settings.trimPercent)) && runner.responsivenessWindow == nil {
						runner.setResponsivenessWindow(intervalIndex)
						phaseCancel()
					}
				}
			}
			prevIntervalBytes = currentBytes
			prevIntervalTime = now
			intervalIndex++
			runner.currentInterval.Store(int64(intervalIndex))
		case <-progressTicker.C:
			if onProgress != nil {
				onProgress(int64(ewmaCapacity), runner.currentRPM.Load())
			}
		case <-phaseCtx.Done():
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			totalDuration := time.Since(start)
			return &directionMeasurement{
				capacity:         runner.finalCapacity(totalDuration),
				rpm:              runner.finalRPM(),
				capacityAccuracy: throughputTracker.accuracy(),
				rpmAccuracy:      responsivenessTracker.accuracy(),
			}, nil
		}
	}
}

func splitRuntimeBudget(total time.Duration, directions int) (time.Duration, time.Duration) {
	if directions <= 1 {
		return total, total
	}
	first := total / time.Duration(directions)
	second := total - first
	return first, second
}

func collectProbeRound(ctx context.Context, foreignClient *http.Client, selfClient *http.Client, rawURL string) (probeMeasurement, error) {
	var foreignResult probeMeasurement
	var selfResult probeMeasurement
	var foreignErr error
	var selfErr error

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		foreignResult, foreignErr = runProbe(ctx, foreignClient, rawURL, false)
	}()
	go func() {
		defer wg.Done()
		selfResult, selfErr = runProbe(ctx, selfClient, rawURL, true)
	}()
	wg.Wait()

	if foreignErr != nil {
		return probeMeasurement{}, E.Cause(foreignErr, "foreign probe")
	}
	if selfErr != nil {
		return probeMeasurement{}, E.Cause(selfErr, "self probe")
	}
	return probeMeasurement{
		tcp:        foreignResult.tcp,
		tls:        foreignResult.tls,
		httpFirst:  foreignResult.httpFirst,
		httpLoaded: selfResult.httpLoaded,
	}, nil
}

func runProbe(ctx context.Context, client *http.Client, rawURL string, expectReuse bool) (probeMeasurement, error) {
	var trace probeTrace
	start := time.Now()
	req, err := newRequest(httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		ConnectStart: func(string, string) {
			if trace.connectStart.IsZero() {
				trace.connectStart = time.Now()
			}
		},
		ConnectDone: func(string, string, error) {
			if trace.connectDone.IsZero() {
				trace.connectDone = time.Now()
			}
		},
		TLSHandshakeStart: func() {
			if trace.tlsStart.IsZero() {
				trace.tlsStart = time.Now()
			}
		},
		TLSHandshakeDone: func(state tls.ConnectionState, _ error) {
			if trace.tlsDone.IsZero() {
				trace.tlsDone = time.Now()
				trace.tlsVersion = state.Version
			}
		},
		GotConn: func(info httptrace.GotConnInfo) {
			trace.reused = info.Reused
			trace.gotConn = time.Now()
		},
		WroteRequest: func(httptrace.WroteRequestInfo) {
			trace.wroteRequest = time.Now()
		},
		GotFirstResponseByte: func() {
			trace.firstResponseByte = time.Now()
		},
	}), http.MethodGet, rawURL, nil)
	if err != nil {
		return probeMeasurement{}, err
	}
	if !expectReuse {
		req.Close = true
	}
	resp, err := client.Do(req)
	if err != nil {
		return probeMeasurement{}, err
	}
	defer resp.Body.Close()
	if err = validateResponse(resp); err != nil {
		return probeMeasurement{}, err
	}
	n, err := io.Copy(io.Discard, resp.Body)
	end := time.Now()
	if err != nil {
		return probeMeasurement{}, err
	}
	if expectReuse && !trace.reused {
		return probeMeasurement{}, E.New("self probe did not reuse an existing connection")
	}

	httpStart := trace.wroteRequest
	if httpStart.IsZero() {
		switch {
		case !trace.tlsDone.IsZero():
			httpStart = trace.tlsDone
		case !trace.connectDone.IsZero():
			httpStart = trace.connectDone
		case !trace.gotConn.IsZero():
			httpStart = trace.gotConn
		default:
			httpStart = start
		}
	}

	measurement := probeMeasurement{
		total:  end.Sub(start),
		bytes:  n,
		reused: trace.reused,
	}
	if !trace.connectStart.IsZero() && !trace.connectDone.IsZero() && trace.connectDone.After(trace.connectStart) {
		measurement.tcp = trace.connectDone.Sub(trace.connectStart)
	}
	if !trace.tlsStart.IsZero() && !trace.tlsDone.IsZero() && trace.tlsDone.After(trace.tlsStart) {
		measurement.tls = trace.tlsDone.Sub(trace.tlsStart)
		if roundTrips := tlsHandshakeRoundTrips(trace.tlsVersion); roundTrips > 1 {
			measurement.tls /= time.Duration(roundTrips)
		}
	}
	if !trace.firstResponseByte.IsZero() && trace.firstResponseByte.After(httpStart) {
		measurement.httpFirst = trace.firstResponseByte.Sub(httpStart)
	}
	if end.After(httpStart) {
		measurement.httpLoaded = end.Sub(httpStart)
	}
	return measurement, nil
}

func runDownloadRequest(ctx context.Context, client *http.Client, rawURL string, onActive func()) error {
	req, err := newRequest(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	err = validateResponse(resp)
	if err != nil {
		return err
	}
	if onActive != nil {
		onActive()
	}
	_, err = sBufio.Copy(io.Discard, resp.Body)
	if ctx.Err() != nil {
		return nil
	}
	return err
}

func runUploadRequest(ctx context.Context, client *http.Client, rawURL string, onActive func()) error {
	body := &uploadBody{
		ctx:      ctx,
		onActive: onActive,
	}
	req, err := newRequest(ctx, http.MethodPost, rawURL, body)
	if err != nil {
		return err
	}
	req.ContentLength = -1
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	defer resp.Body.Close()
	err = validateResponse(resp)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	<-ctx.Done()
	return nil
}

func newRequest(ctx context.Context, method string, rawURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	return req, nil
}

func validateResponse(resp *http.Response) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return E.New("unexpected status: ", resp.Status)
	}
	if encoding := resp.Header.Get("Content-Encoding"); encoding != "" {
		return E.New("unexpected content encoding: ", encoding)
	}
	return nil
}

func calculateRPM(rounds []probeRound) int32 {
	if len(rounds) == 0 {
		return 0
	}
	var tcpSamples []float64
	var tlsSamples []float64
	var httpFirstSamples []float64
	var httpLoadedSamples []float64
	for _, round := range rounds {
		if round.tcp > 0 {
			tcpSamples = append(tcpSamples, durationMillis(round.tcp))
		}
		if round.tls > 0 {
			tlsSamples = append(tlsSamples, durationMillis(round.tls))
		}
		if round.httpFirst > 0 {
			httpFirstSamples = append(httpFirstSamples, durationMillis(round.httpFirst))
		}
		if round.httpLoaded > 0 {
			httpLoadedSamples = append(httpLoadedSamples, durationMillis(round.httpLoaded))
		}
	}
	httpLoaded := upperTrimmedMean(httpLoadedSamples, settings.trimPercent)
	if httpLoaded <= 0 {
		return 0
	}
	var foreignComponents []float64
	if tcp := upperTrimmedMean(tcpSamples, settings.trimPercent); tcp > 0 {
		foreignComponents = append(foreignComponents, tcp)
	}
	if tls := upperTrimmedMean(tlsSamples, settings.trimPercent); tls > 0 {
		foreignComponents = append(foreignComponents, tls)
	}
	if httpFirst := upperTrimmedMean(httpFirstSamples, settings.trimPercent); httpFirst > 0 {
		foreignComponents = append(foreignComponents, httpFirst)
	}
	if len(foreignComponents) == 0 {
		return 0
	}
	foreignLatency := meanFloat64s(foreignComponents)
	foreignRPM := 60000.0 / foreignLatency
	loadedRPM := 60000.0 / httpLoaded
	return int32(math.Round((foreignRPM + loadedRPM) / 2))
}

func tlsHandshakeRoundTrips(version uint16) int {
	switch version {
	case tls.VersionTLS12, tls.VersionTLS11, tls.VersionTLS10:
		return 2
	default:
		return 1
	}
}

func durationMillis(value time.Duration) float64 {
	return float64(value) / float64(time.Millisecond)
}

func upperTrimmedMean(values []float64, trimPercent int) float64 {
	trimmed := upperTrimFloat64s(values, trimPercent)
	if len(trimmed) == 0 {
		return 0
	}
	return meanFloat64s(trimmed)
}

func upperTrimFloat64s(values []float64, trimPercent int) []float64 {
	if len(values) == 0 {
		return nil
	}
	trimmed := append([]float64(nil), values...)
	sort.Float64s(trimmed)
	if trimPercent <= 0 {
		return trimmed
	}
	trimCount := int(math.Floor(float64(len(trimmed)) * float64(trimPercent) / 100))
	if trimCount <= 0 || trimCount >= len(trimmed) {
		return trimmed
	}
	return trimmed[:len(trimmed)-trimCount]
}

func meanFloat64s(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var total float64
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func stdDevFloat64s(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	mean := meanFloat64s(values)
	var total float64
	for _, value := range values {
		delta := value - mean
		total += delta * delta
	}
	return math.Sqrt(total / float64(len(values)))
}

type uploadBody struct {
	ctx       context.Context
	activated atomic.Bool
	onActive  func()
}

func (u *uploadBody) Read(p []byte) (int, error) {
	if err := u.ctx.Err(); err != nil {
		return 0, err
	}
	clear(p)
	n := len(p)
	if n > 0 && u.onActive != nil && u.activated.CompareAndSwap(false, true) {
		u.onActive()
	}
	return n, nil
}

func (u *uploadBody) Close() error {
	return nil
}
