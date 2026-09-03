package oomkiller

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"runtime/metrics"
	"strconv"
	"sync"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/memory"
)

const (
	DraftDirectoryName   = "oom_draft"
	ReportsDirectoryName = "oom_reports"

	timelineFileName = "timeline.jsonl"
	eventsFileName   = "events.jsonl"
	metadataFileName = "metadata.json"
	logFileName      = "go.log"
	lockFileName     = ".lock"

	normalSampleInterval   = time.Minute
	pressureSampleInterval = time.Second
	denseSampleWindow      = 2 * time.Minute

	pressureSnapshotMinInterval = time.Hour
	resetSnapshotMinInterval    = 10 * time.Minute
	maxAutomaticSnapshots       = 8
)

const (
	SnapshotReasonPressure = "pressure"
	SnapshotReasonReset    = "reset"
	SnapshotReasonManual   = "manual"

	resetReasonThreshold = "threshold"
	resetReasonRate      = "rate"

	eventTypeStart    = "start"
	eventTypeStop     = "stop"
	eventTypePressure = "pressure"
	eventTypeState    = "state"
	eventTypeReset    = "reset"
	eventTypeSnapshot = "snapshot"
	eventTypeReport   = "report"
)

type RecorderOptions struct {
	BasePath         string
	Logger           logger.Logger
	AcceptDraft      func(metadataContent []byte) bool
	MetadataCallback func(status ReportStatus) any
	OwnerCallback    func(path string)
	LogCallback      func() []byte
	SnapshotCallback func(directory string, prefix string)
}

type ReportStatus struct {
	StartedAt      time.Time
	RecordedAt     time.Time
	EndedAt        time.Time
	MemoryLimit    uint64
	PeakMemory     uint64
	MinAvailable   uint64
	AvailableKnown bool
	Snapshots      int
}

type Recorder struct {
	basePath         string
	draftPath        string
	logger           logger.Logger
	acceptDraft      func(metadataContent []byte) bool
	metadataCallback func(status ReportStatus) any
	ownerCallback    func(path string)
	logCallback      func() []byte
	snapshotCallback func(directory string, prefix string)

	access           sync.Mutex
	started          bool
	closed           bool
	draftCreated     bool
	draftLock        *os.File
	notable          bool
	status           ReportStatus
	lastRowAt        time.Time
	lastRowState     pressureState
	hasRow           bool
	denseUntil       time.Time
	previousGCCycles uint64
	metricsSamples   []metrics.Sample

	snapshotAccess     sync.Mutex
	snapshotSequence   int
	automaticSnapshots int
	lastSnapshotAt     map[string]time.Time
}

type timelineRow struct {
	At              string `json:"at"`
	State           string `json:"state"`
	MemoryBytes     uint64 `json:"memoryBytes"`
	AvailableBytes  uint64 `json:"availableBytes,omitempty"`
	GoMemoryBytes   uint64 `json:"goMemoryBytes,omitempty"`
	GoHeapLiveBytes uint64 `json:"goHeapLiveBytes,omitempty"`
	GoStackBytes    uint64 `json:"goStackBytes,omitempty"`
	Goroutines      uint64 `json:"goroutines,omitempty"`
	GCCycles        uint64 `json:"gcCycles,omitempty"`
	Connections     int    `json:"connections,omitempty"`
}

type eventRecord struct {
	Type             string        `json:"t"`
	At               string        `json:"at"`
	Policy           string        `json:"policy,omitempty"`
	State            string        `json:"state,omitempty"`
	Reason           string        `json:"reason,omitempty"`
	MemoryBytes      uint64        `json:"memoryBytes,omitempty"`
	AvailableBytes   uint64        `json:"availableBytes,omitempty"`
	MemoryAfterBytes uint64        `json:"memoryAfterBytes,omitempty"`
	MemoryLimit      uint64        `json:"memoryLimit,omitempty"`
	TriggerBytes     uint64        `json:"triggerBytes,omitempty"`
	ArmedBytes       uint64        `json:"armedBytes,omitempty"`
	ResumeBytes      uint64        `json:"resumeBytes,omitempty"`
	ReportOnly       bool          `json:"reportOnly,omitempty"`
	Connections      int           `json:"connections,omitempty"`
	Sequence         int           `json:"seq,omitempty"`
	Prefix           string        `json:"prefix,omitempty"`
	Runtime          *runtimeStats `json:"runtime,omitempty"`
}

type runtimeStats struct {
	HeapAlloc    uint64 `json:"heapAlloc"`
	HeapObjects  uint64 `json:"heapObjects"`
	HeapInuse    uint64 `json:"heapInuse"`
	HeapIdle     uint64 `json:"heapIdle"`
	HeapReleased uint64 `json:"heapReleased"`
	HeapSys      uint64 `json:"heapSys"`
	StackInuse   uint64 `json:"stackInuse"`
	StackSys     uint64 `json:"stackSys"`
	Sys          uint64 `json:"sys"`
	TotalAlloc   uint64 `json:"totalAlloc"`
	NumGC        uint32 `json:"numGC"`
	NumGoroutine int    `json:"numGoroutine"`
	NextGC       uint64 `json:"nextGC"`
	LastGC       string `json:"lastGC,omitempty"`
}

func NewRecorder(options RecorderOptions) *Recorder {
	recorderLogger := options.Logger
	if recorderLogger == nil {
		recorderLogger = logger.NOP()
	}
	return &Recorder{
		basePath:         options.BasePath,
		draftPath:        filepath.Join(options.BasePath, DraftDirectoryName),
		logger:           recorderLogger,
		acceptDraft:      options.AcceptDraft,
		metadataCallback: options.MetadataCallback,
		ownerCallback:    options.OwnerCallback,
		logCallback:      options.LogCallback,
		snapshotCallback: options.SnapshotCallback,
		metricsSamples: []metrics.Sample{
			{Name: "/memory/classes/total:bytes"},
			{Name: "/gc/heap/live:bytes"},
			{Name: "/memory/classes/heap/stacks:bytes"},
			{Name: "/sched/goroutines:goroutines"},
			{Name: "/gc/cycles/total:gc-cycles"},
		},
		lastSnapshotAt: make(map[string]time.Time),
	}
}

func (r *Recorder) Start() {
	r.access.Lock()
	defer r.access.Unlock()
	if r.started {
		return
	}
	PromoteDraft(r.basePath, r.acceptDraft)
	r.started = true
	r.status.StartedAt = time.Now()
}

func (r *Recorder) Close() error {
	r.snapshotAccess.Lock()
	defer r.snapshotAccess.Unlock()
	r.access.Lock()
	defer r.access.Unlock()
	if !r.started || r.closed {
		return nil
	}
	r.closed = true
	if !r.draftCreated {
		return nil
	}
	if !r.notable {
		r.releaseDraftLocked()
		return os.RemoveAll(r.draftPath)
	}
	r.status.EndedAt = time.Now()
	r.writeLogLocked()
	r.writeMetadataLocked()
	r.releaseDraftLocked()
	promoteDirectory(r.draftPath, filepath.Join(r.basePath, ReportsDirectoryName))
	return nil
}

func (r *Recorder) releaseDraftLocked() {
	if r.draftLock == nil {
		return
	}
	os.Remove(r.draftLock.Name())
	unlockDraft(r.draftLock)
	r.draftLock = nil
}

func (r *Recorder) WriteReport() error {
	sample := memorySample{usage: memory.Total()}
	if memory.AvailableAvailable() {
		sample.availableKnown = true
		sample.available = memory.Available()
	}
	err := r.snapshot(SnapshotReasonManual, sample, true)
	if err != nil {
		return E.Cause(err, "write snapshot")
	}
	r.access.Lock()
	defer r.access.Unlock()
	reportsDir := filepath.Join(r.basePath, ReportsDirectoryName)
	err = os.MkdirAll(reportsDir, 0o777)
	if err != nil {
		return E.Cause(err, "create reports directory")
	}
	r.chown(reportsDir)
	destPath, err := nextAvailableReportPath(reportsDir, time.Now().UTC())
	if err != nil {
		return err
	}
	r.appendEventLocked(eventRecord{Type: eventTypeReport, MemoryBytes: sample.usage, AvailableBytes: sample.available})
	err = copyDirectory(r.draftPath, destPath)
	if err != nil {
		os.RemoveAll(destPath)
		return E.Cause(err, "copy draft to ", destPath)
	}
	r.chownTree(destPath)
	return nil
}

func (r *Recorder) instanceStarted(config timerConfig, thresholds pressureThresholds) {
	r.access.Lock()
	defer r.access.Unlock()
	r.status.MemoryLimit = config.memoryLimit
	r.appendEventLocked(eventRecord{
		Type:         eventTypeStart,
		Policy:       config.policyMode.String(),
		MemoryLimit:  config.memoryLimit,
		TriggerBytes: thresholds.trigger,
		ArmedBytes:   thresholds.armed,
		ResumeBytes:  thresholds.resume,
		ReportOnly:   config.killerDisabled,
	})
}

func (r *Recorder) instanceStopped() {
	r.access.Lock()
	defer r.access.Unlock()
	r.appendEventLocked(eventRecord{Type: eventTypeStop})
}

func (r *Recorder) sample(sample memorySample, state pressureState, connections int) {
	now := time.Now()
	r.access.Lock()
	defer r.access.Unlock()
	if r.closed {
		return
	}
	r.observeLocked(sample)
	if r.hasRow && state == r.lastRowState {
		interval := normalSampleInterval
		if now.Before(r.denseUntil) {
			interval = pressureSampleInterval
		}
		if now.Sub(r.lastRowAt) < interval {
			return
		}
	}
	err := r.ensureDraftLocked()
	if err != nil {
		return
	}
	metrics.Read(r.metricsSamples)
	gcCycles := r.metricsSamples[4].Value.Uint64()
	row := timelineRow{
		At:              now.UTC().Format(time.RFC3339),
		State:           state.String(),
		MemoryBytes:     sample.usage,
		AvailableBytes:  sample.available,
		GoMemoryBytes:   r.metricsSamples[0].Value.Uint64(),
		GoHeapLiveBytes: r.metricsSamples[1].Value.Uint64(),
		GoStackBytes:    r.metricsSamples[2].Value.Uint64(),
		Goroutines:      r.metricsSamples[3].Value.Uint64(),
		Connections:     connections,
	}
	if r.hasRow {
		row.GCCycles = gcCycles - r.previousGCCycles
	}
	r.previousGCCycles = gcCycles
	r.hasRow = true
	r.lastRowAt = now
	r.lastRowState = state
	timelinePath := filepath.Join(r.draftPath, timelineFileName)
	err = appendRecord(timelinePath, row)
	if err != nil {
		r.logger.Error(E.Cause(err, "OOM report: write timeline"))
		return
	}
	r.chown(timelinePath)
}

//nolint:unused
func (r *Recorder) recordPressure(sample memorySample) {
	r.access.Lock()
	defer r.access.Unlock()
	r.notable = true
	r.observeLocked(sample)
	r.denseUntil = time.Now().Add(denseSampleWindow)
	r.appendEventLocked(eventRecord{Type: eventTypePressure, MemoryBytes: sample.usage, AvailableBytes: sample.available})
}

func (r *Recorder) recordStateChange(state pressureState, sample memorySample) {
	r.access.Lock()
	defer r.access.Unlock()
	r.observeLocked(sample)
	r.denseUntil = time.Now().Add(denseSampleWindow)
	r.appendEventLocked(eventRecord{Type: eventTypeState, State: state.String(), MemoryBytes: sample.usage, AvailableBytes: sample.available})
}

func (r *Recorder) recordReset(reason string, before memorySample, after memorySample, connections int, reportOnly bool) {
	r.access.Lock()
	defer r.access.Unlock()
	r.notable = true
	r.observeLocked(before)
	r.denseUntil = time.Now().Add(denseSampleWindow)
	r.appendEventLocked(eventRecord{
		Type:             eventTypeReset,
		Reason:           reason,
		MemoryBytes:      before.usage,
		AvailableBytes:   before.available,
		MemoryAfterBytes: after.usage,
		Connections:      connections,
		ReportOnly:       reportOnly,
	})
}

func (r *Recorder) snapshot(reason string, sample memorySample, force bool) error {
	r.snapshotAccess.Lock()
	defer r.snapshotAccess.Unlock()
	now := time.Now()
	if !force {
		if r.automaticSnapshots >= maxAutomaticSnapshots {
			return nil
		}
		var minInterval time.Duration
		switch reason {
		case SnapshotReasonPressure:
			minInterval = pressureSnapshotMinInterval
		case SnapshotReasonReset:
			minInterval = resetSnapshotMinInterval
		}
		lastAt, found := r.lastSnapshotAt[reason]
		if found && now.Sub(lastAt) < minInterval {
			return nil
		}
	}
	r.access.Lock()
	if r.closed {
		r.access.Unlock()
		return E.New("OOM recorder closed")
	}
	err := r.ensureDraftLocked()
	r.access.Unlock()
	if err != nil {
		return err
	}
	r.snapshotSequence++
	sequence := r.snapshotSequence
	if !force {
		r.automaticSnapshots++
	}
	r.lastSnapshotAt[reason] = now
	prefix := "snapshot-" + strconv.Itoa(sequence) + "-" + reason
	if r.snapshotCallback != nil {
		r.snapshotCallback(r.draftPath, prefix)
	}
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	stats := &runtimeStats{
		HeapAlloc:    memStats.HeapAlloc,
		HeapObjects:  memStats.HeapObjects,
		HeapInuse:    memStats.HeapInuse,
		HeapIdle:     memStats.HeapIdle,
		HeapReleased: memStats.HeapReleased,
		HeapSys:      memStats.HeapSys,
		StackInuse:   memStats.StackInuse,
		StackSys:     memStats.StackSys,
		Sys:          memStats.Sys,
		TotalAlloc:   memStats.TotalAlloc,
		NumGC:        memStats.NumGC,
		NumGoroutine: runtime.NumGoroutine(),
		NextGC:       memStats.NextGC,
	}
	if memStats.LastGC > 0 {
		stats.LastGC = time.Unix(0, int64(memStats.LastGC)).UTC().Format(time.RFC3339)
	}
	r.access.Lock()
	defer r.access.Unlock()
	r.notable = true
	r.status.Snapshots = sequence
	r.observeLocked(sample)
	r.writeLogLocked()
	r.appendEventLocked(eventRecord{
		Type:           eventTypeSnapshot,
		Reason:         reason,
		Sequence:       sequence,
		Prefix:         prefix,
		MemoryBytes:    sample.usage,
		AvailableBytes: sample.available,
		Runtime:        stats,
	})
	return nil
}

func (r *Recorder) observeLocked(sample memorySample) {
	if sample.usage > r.status.PeakMemory {
		r.status.PeakMemory = sample.usage
	}
	if sample.availableKnown && (!r.status.AvailableKnown || sample.available < r.status.MinAvailable) {
		r.status.AvailableKnown = true
		r.status.MinAvailable = sample.available
	}
}

func (r *Recorder) ensureDraftLocked() error {
	if r.draftCreated {
		_, err := os.Stat(r.draftPath)
		if err == nil {
			return nil
		}
		r.logger.Error("OOM report: draft directory lost, recreating")
		r.releaseDraftLocked()
		r.draftCreated = false
		r.hasRow = false
	}
	if !r.started {
		return E.New("OOM recorder not started")
	}
	if r.closed {
		return E.New("OOM recorder closed")
	}
	err := os.MkdirAll(r.draftPath, 0o777)
	if err != nil {
		r.logger.Error(E.Cause(err, "OOM report: create draft directory"))
		return E.Cause(err, "create draft directory ", r.draftPath)
	}
	r.chown(r.draftPath)
	lockPath := filepath.Join(r.draftPath, lockFileName)
	r.draftLock, err = lockDraft(lockPath)
	if err != nil {
		return E.Cause(err, "lock draft directory ", r.draftPath)
	}
	r.chown(lockPath)
	r.draftCreated = true
	r.writeMetadataLocked()
	return nil
}

func (r *Recorder) appendEventLocked(event eventRecord) {
	if r.closed {
		return
	}
	err := r.ensureDraftLocked()
	if err != nil {
		return
	}
	event.At = time.Now().UTC().Format(time.RFC3339)
	eventsPath := filepath.Join(r.draftPath, eventsFileName)
	err = appendRecord(eventsPath, event)
	if err != nil {
		r.logger.Error(E.Cause(err, "OOM report: write events"))
	} else {
		r.chown(eventsPath)
	}
	r.writeMetadataLocked()
}

func (r *Recorder) writeMetadataLocked() {
	if r.metadataCallback == nil {
		return
	}
	r.status.RecordedAt = time.Now()
	content, err := json.Marshal(r.metadataCallback(r.status))
	if err != nil {
		return
	}
	r.writeFile(filepath.Join(r.draftPath, metadataFileName), content)
}

func (r *Recorder) writeLogLocked() {
	if r.logCallback == nil {
		return
	}
	content := r.logCallback()
	if len(content) == 0 {
		return
	}
	r.writeFile(filepath.Join(r.draftPath, logFileName), content)
}

func (r *Recorder) writeFile(path string, content []byte) {
	err := os.WriteFile(path, content, 0o666)
	if err != nil {
		r.logger.Error(E.Cause(err, "OOM report: write ", filepath.Base(path)))
		return
	}
	r.chown(path)
}

func (r *Recorder) chown(path string) {
	if r.ownerCallback != nil {
		r.ownerCallback(path)
	}
}

func (r *Recorder) chownTree(directory string) {
	r.chown(directory)
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	for _, entry := range entries {
		r.chown(filepath.Join(directory, entry.Name()))
	}
}

func appendRecord(path string, record any) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(record)
}
