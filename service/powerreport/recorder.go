package powerreport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/metrics"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/memory"
)

const (
	DraftDirectoryName   = "power_draft"
	ReportsDirectoryName = "power_reports"

	timelineFileName         = "timeline.jsonl"
	eventsFileName           = "events.jsonl"
	metadataFileName         = "metadata.json"
	logFileName              = "go.log"
	goroutineProfileFileName = "goroutine.pb.gz"

	defaultGateInterval     = 5 * time.Second
	defaultSampleInterval   = time.Minute
	defaultFlushInterval    = 15 * time.Minute
	defaultFallbackInterval = 10 * time.Minute

	activityRefreshNano = int64(time.Second)

	rowCapacity   = 4096
	eventCapacity = 8192
)

type Options struct {
	BasePath         string
	Logger           logger.Logger
	Metadata         any
	OwnerCallback    func(path string)
	LogCallback      func() []byte
	ProfileCallback  func(path string)
	GateInterval     time.Duration
	SampleInterval   time.Duration
	FlushInterval    time.Duration
	FallbackInterval time.Duration
}

type Recorder struct {
	draftPath        string
	logger           logger.Logger
	metadata         any
	ownerCallback    func(path string)
	logCallback      func() []byte
	profileCallback  func(path string)
	gateNano         int64
	sampleNano       int64
	flushInterval    time.Duration
	fallbackInterval time.Duration
	baseTime         time.Time

	_            [64]byte
	lastActivity atomic.Int64
	_            [64]byte

	lastSampleAt atomic.Int64
	pendingBreak atomic.Pointer[breakRecord]
	notify       chan struct{}

	dnsQueries        atomic.Uint64
	connectionsOpened atomic.Uint64

	access         sync.Mutex
	networkType    string
	rows           []timelineRow
	events         []eventRecord
	previous       previousSample
	lastFlushAt    time.Time
	started        bool
	closed         bool
	metricsSamples []metrics.Sample

	done       chan struct{}
	workerDone chan struct{}
}

type breakRecord struct {
	at        time.Time
	idleMS    int64
	direction Direction
	size      int
	by        *Attribution
}

type previousSample struct {
	at                time.Time
	usage             systemUsage
	gcSeconds         float64
	gcCycles          uint64
	absoluteTime      int64
	continuousTime    int64
	interfaces        map[string]interfaceCounters
	dnsQueries        uint64
	connectionsOpened uint64
}

func NewRecorder(options Options) *Recorder {
	recorderLogger := options.Logger
	if recorderLogger == nil {
		recorderLogger = logger.NOP()
	}
	gateInterval := options.GateInterval
	if gateInterval == 0 {
		gateInterval = defaultGateInterval
	}
	sampleInterval := options.SampleInterval
	if sampleInterval == 0 {
		sampleInterval = defaultSampleInterval
	}
	flushInterval := options.FlushInterval
	if flushInterval == 0 {
		flushInterval = defaultFlushInterval
	}
	fallbackInterval := options.FallbackInterval
	if fallbackInterval == 0 {
		fallbackInterval = defaultFallbackInterval
	}
	return &Recorder{
		draftPath:        filepath.Join(options.BasePath, DraftDirectoryName),
		logger:           recorderLogger,
		metadata:         options.Metadata,
		ownerCallback:    options.OwnerCallback,
		logCallback:      options.LogCallback,
		profileCallback:  options.ProfileCallback,
		gateNano:         int64(gateInterval),
		sampleNano:       int64(sampleInterval),
		flushInterval:    flushInterval,
		fallbackInterval: fallbackInterval,
		baseTime:         time.Now(),
		notify:           make(chan struct{}, 1),
		metricsSamples: []metrics.Sample{
			{Name: "/cpu/classes/gc/total:cpu-seconds"},
			{Name: "/sched/goroutines:goroutines"},
			{Name: "/gc/cycles/total:gc-cycles"},
			{Name: "/memory/classes/total:bytes"},
			{Name: "/gc/heap/live:bytes"},
		},
		done:       make(chan struct{}),
		workerDone: make(chan struct{}),
	}
}

func (r *Recorder) Start() error {
	r.access.Lock()
	defer r.access.Unlock()
	if r.started {
		return nil
	}
	PromoteDraft(filepath.Dir(r.draftPath))
	err := os.MkdirAll(r.draftPath, 0o777)
	if err != nil {
		return E.Cause(err, "create power report draft directory")
	}
	r.chown(r.draftPath)
	if r.metadata != nil {
		metadataContent, marshalErr := json.Marshal(r.metadata)
		if marshalErr == nil {
			metadataPath := filepath.Join(r.draftPath, metadataFileName)
			os.WriteFile(metadataPath, metadataContent, 0o666)
			r.chown(metadataPath)
		}
	}
	now := time.Now()
	r.resetPreviousLocked(now)
	r.lastSampleAt.Store(int64(now.Sub(r.baseTime)))
	r.lastFlushAt = now
	r.started = true
	go r.worker()
	return nil
}

func (r *Recorder) Close() error {
	r.access.Lock()
	if !r.started || r.closed {
		r.access.Unlock()
		return nil
	}
	r.closed = true
	r.access.Unlock()
	close(r.done)
	<-r.workerDone
	now := time.Now()
	r.access.Lock()
	r.consumeBreakLocked()
	r.sampleLocked(now)
	r.flushLocked(now)
	r.access.Unlock()
	r.writeProfiles()
	r.writeLog()
	finalizeDraft(r.draftPath)
	return nil
}

func (r *Recorder) writeLog() {
	if r.logCallback == nil {
		return
	}
	content := r.logCallback()
	if len(content) == 0 {
		return
	}
	logPath := filepath.Join(r.draftPath, logFileName)
	err := os.WriteFile(logPath, content, 0o666)
	if err != nil {
		return
	}
	r.chown(logPath)
}

// Touch reports one I/O activity: one read or write call, or one batched receive or send
// syscall on paths that batch packets. size is the size of the first packet of the activity
// and characterizes what ended an idle period; it is not accumulated. Volume totals come from
// the sampled interface counters instead.
func (r *Recorder) Touch(direction Direction, size int, by *Attribution) {
	nowNano := int64(time.Since(r.baseTime))
	lastNano := r.lastActivity.Load()
	if nowNano-lastNano < activityRefreshNano {
		return
	}
	previousNano := r.lastActivity.Swap(nowNano)
	if previousNano != 0 && nowNano-previousNano >= r.gateNano {
		r.pendingBreak.Store(&breakRecord{
			at:        time.Now(),
			idleMS:    (nowNano - previousNano) / int64(time.Millisecond),
			direction: direction,
			size:      size,
			by:        by,
		})
		r.notifyWorker()
	} else if nowNano-r.lastSampleAt.Load() >= r.sampleNano {
		r.notifyWorker()
	}
}

func (r *Recorder) CountDNSQuery() {
	r.dnsQueries.Add(1)
}

func (r *Recorder) CountConnectionOpened() {
	r.connectionsOpened.Add(1)
}

func (r *Recorder) RecordPlatformEvent(eventType string) {
	now := time.Now()
	r.access.Lock()
	if !r.started || r.closed {
		r.access.Unlock()
		return
	}
	r.events = append(r.events, eventRecord{
		Type: eventType,
		At:   now.UTC().Format(time.RFC3339),
	})
	r.access.Unlock()
	r.notifyWorker()
}

func (r *Recorder) UpdateNetworkType(networkType string) {
	now := time.Now()
	r.access.Lock()
	if r.closed || r.networkType == networkType {
		r.access.Unlock()
		return
	}
	r.networkType = networkType
	r.events = append(r.events, eventRecord{
		Type:        eventTypeNetwork,
		At:          now.UTC().Format(time.RFC3339),
		NetworkType: networkType,
	})
	r.access.Unlock()
	r.notifyWorker()
}

func (r *Recorder) notifyWorker() {
	select {
	case r.notify <- struct{}{}:
	default:
	}
}

func (r *Recorder) worker() {
	defer close(r.workerDone)
	timer := time.NewTimer(r.fallbackInterval)
	defer timer.Stop()
	for {
		select {
		case <-r.done:
			return
		case <-r.notify:
		case <-timer.C:
			timer.Reset(r.fallbackInterval)
		}
		r.process()
	}
}

func (r *Recorder) process() {
	now := time.Now()
	r.access.Lock()
	defer r.access.Unlock()
	if !r.started || r.closed {
		return
	}
	r.consumeBreakLocked()
	nowNano := int64(now.Sub(r.baseTime))
	if nowNano-r.lastSampleAt.Load() >= r.sampleNano {
		r.lastSampleAt.Store(nowNano)
		r.sampleLocked(now)
	}
	if now.Sub(r.lastFlushAt) >= r.flushInterval || len(r.rows) >= rowCapacity || len(r.events) >= eventCapacity {
		r.flushLocked(now)
	}
}

func (r *Recorder) consumeBreakLocked() {
	record := r.pendingBreak.Swap(nil)
	if record == nil {
		return
	}
	r.events = append(r.events, eventRecord{
		Type:        eventTypeBreak,
		At:          record.at.UTC().Format(time.RFC3339),
		IdleMS:      record.idleMS,
		Direction:   record.direction.String(),
		Size:        record.size,
		NetworkType: r.networkType,
		By:          record.by,
	})
}

func (r *Recorder) resetPreviousLocked(now time.Time) {
	metrics.Read(r.metricsSamples)
	r.previous = previousSample{
		at:                now,
		usage:             readSystemUsage(),
		gcSeconds:         r.metricsSamples[0].Value.Float64(),
		gcCycles:          r.metricsSamples[2].Value.Uint64(),
		interfaces:        readInterfaceCounters(),
		dnsQueries:        r.dnsQueries.Load(),
		connectionsOpened: r.connectionsOpened.Load(),
	}
	r.previous.absoluteTime, r.previous.continuousTime = readClocks()
}

func (r *Recorder) sampleLocked(now time.Time) {
	previous := r.previous
	r.resetPreviousLocked(now)
	current := &r.previous
	row := timelineRow{
		From:              previous.at.UTC().Format(time.RFC3339),
		To:                now.UTC().Format(time.RFC3339),
		CPUGCMS:           int64((current.gcSeconds - previous.gcSeconds) * 1000),
		Goroutines:        r.metricsSamples[1].Value.Uint64(),
		GCCycles:          current.gcCycles - previous.gcCycles,
		GoMemoryBytes:     r.metricsSamples[3].Value.Uint64(),
		GoHeapLiveBytes:   r.metricsSamples[4].Value.Uint64(),
		DNSQueries:        current.dnsQueries - previous.dnsQueries,
		ConnectionsOpened: current.connectionsOpened - previous.connectionsOpened,
		NetworkType:       r.networkType,
	}
	if memory.TotalAvailable() {
		row.MemoryBytes = memory.Total()
	}
	if current.usage.valid && previous.usage.valid {
		row.CPUUserMS = (current.usage.userTime - previous.usage.userTime) / int64(time.Millisecond)
		row.CPUSystemMS = (current.usage.systemTime - previous.usage.systemTime) / int64(time.Millisecond)
		row.CPUPerformanceMS = (current.usage.performanceUserTime - previous.usage.performanceUserTime +
			current.usage.performanceSystemTime - previous.usage.performanceSystemTime) / int64(time.Millisecond)
		row.PackageIdleWakeups = current.usage.packageIdleWakeups - previous.usage.packageIdleWakeups
		row.InterruptWakeups = current.usage.interruptWakeups - previous.usage.interruptWakeups
		row.EnergyNanojoules = current.usage.energyNanojoules - previous.usage.energyNanojoules
		row.PerformanceEnergyNanojoules = current.usage.performanceEnergyNanojoules - previous.usage.performanceEnergyNanojoules
		row.DiskBytesWritten = current.usage.diskBytesWritten - previous.usage.diskBytesWritten
		qos := qosBreakdown{
			DefaultMS:         (current.usage.qosDefaultTime - previous.usage.qosDefaultTime) / int64(time.Millisecond),
			MaintenanceMS:     (current.usage.qosMaintenanceTime - previous.usage.qosMaintenanceTime) / int64(time.Millisecond),
			BackgroundMS:      (current.usage.qosBackgroundTime - previous.usage.qosBackgroundTime) / int64(time.Millisecond),
			UtilityMS:         (current.usage.qosUtilityTime - previous.usage.qosUtilityTime) / int64(time.Millisecond),
			LegacyMS:          (current.usage.qosLegacyTime - previous.usage.qosLegacyTime) / int64(time.Millisecond),
			UserInitiatedMS:   (current.usage.qosUserInitiatedTime - previous.usage.qosUserInitiatedTime) / int64(time.Millisecond),
			UserInteractiveMS: (current.usage.qosUserInteractiveTime - previous.usage.qosUserInteractiveTime) / int64(time.Millisecond),
		}
		if qos != (qosBreakdown{}) {
			row.QoSMS = &qos
		}
	}
	if current.absoluteTime != 0 && previous.absoluteTime != 0 && current.absoluteTime >= previous.absoluteTime {
		sleptNano := (current.continuousTime - previous.continuousTime) - (current.absoluteTime - previous.absoluteTime)
		wallNano := now.Sub(previous.at).Nanoseconds()
		if sleptNano > wallNano {
			sleptNano = wallNano
		}
		if sleptNano > 0 {
			row.SleptMS = sleptNano / int64(time.Millisecond)
		}
	}
	if len(current.interfaces) > 0 && len(previous.interfaces) > 0 {
		interfacePackets := make(map[string]uint64)
		for name, counters := range current.interfaces {
			previousCounters, found := previous.interfaces[name]
			if !found {
				continue
			}
			delta := uint64(counters.inPackets-previousCounters.inPackets) + uint64(counters.outPackets-previousCounters.outPackets)
			if delta > 0 {
				interfacePackets[name] = delta
			}
		}
		if len(interfacePackets) > 0 {
			row.InterfacePackets = interfacePackets
		}
	}
	r.rows = append(r.rows, row)
}

func (r *Recorder) chown(path string) {
	if r.ownerCallback != nil {
		r.ownerCallback(path)
	}
}

func (r *Recorder) flushLocked(now time.Time) {
	err := appendRecords(r, filepath.Join(r.draftPath, timelineFileName), r.rows)
	if err == nil {
		r.rows = r.rows[:0]
	} else {
		r.logger.Error(E.Cause(err, "power report: write timeline"))
		if len(r.rows) >= rowCapacity {
			r.rows = r.rows[len(r.rows)-rowCapacity/2:]
		}
	}
	err = appendRecords(r, filepath.Join(r.draftPath, eventsFileName), r.events)
	if err == nil {
		r.events = r.events[:0]
	} else {
		r.logger.Error(E.Cause(err, "power report: write events"))
		if len(r.events) >= eventCapacity {
			r.events = r.events[len(r.events)-eventCapacity/2:]
		}
	}
	r.lastFlushAt = now
}

func appendRecords[T any](r *Recorder, path string, records []T) error {
	if len(records) == 0 {
		return nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return err
	}
	defer file.Close()
	r.chown(path)
	encoder := json.NewEncoder(file)
	for _, record := range records {
		err = encoder.Encode(record)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Recorder) writeProfiles() {
	if r.profileCallback != nil {
		r.profileCallback(r.draftPath)
		return
	}
	profilePath := filepath.Join(r.draftPath, goroutineProfileFileName)
	file, err := os.OpenFile(profilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o666)
	if err != nil {
		return
	}
	defer file.Close()
	r.chown(profilePath)
	pprof.Lookup("goroutine").WriteTo(file, 0)
}
