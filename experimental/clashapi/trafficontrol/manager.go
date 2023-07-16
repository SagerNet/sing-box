package trafficontrol

import (
	"runtime"
	"time"

	"github.com/sagernet/sing-box/experimental/clashapi/compatible"
	"github.com/sagernet/sing/common/atomic"
)

type Manager struct {
	uploadTemp    atomic.Int64
	downloadTemp  atomic.Int64
	uploadBlip    atomic.Int64
	downloadBlip  atomic.Int64
	uploadTotal   atomic.Int64
	downloadTotal atomic.Int64

	connections compatible.Map[string, tracker]
	ticker      *time.Ticker
	done        chan struct{}
	// process     *process.Process
	memory uint64
}

func NewManager() *Manager {
	manager := &Manager{
		ticker: time.NewTicker(time.Second),
		done:   make(chan struct{}),
		// process: &process.Process{Pid: int32(os.Getpid())},
	}
	go manager.handle()
	return manager
}

func (m *Manager) Join(c tracker) {
	m.connections.Store(c.ID(), c)
}

func (m *Manager) Leave(c tracker) {
	m.connections.Delete(c.ID())
}

func (m *Manager) PushUploaded(size int64) {
	m.uploadTemp.Add(size)
	m.uploadTotal.Add(size)
}

func (m *Manager) PushDownloaded(size int64) {
	m.downloadTemp.Add(size)
	m.downloadTotal.Add(size)
}

func (m *Manager) Now() (up int64, down int64) {
	return m.uploadBlip.Load(), m.downloadBlip.Load()
}

func (m *Manager) Total() (up int64, down int64) {
	return m.uploadTotal.Load(), m.downloadTotal.Load()
}

func (m *Manager) Connections() int {
	return m.connections.Len()
}

func (m *Manager) Snapshot() *Snapshot {
	var connections []tracker
	m.connections.Range(func(_ string, value tracker) bool {
		connections = append(connections, value)
		return true
	})

	//if memoryInfo, err := m.process.MemoryInfo(); err == nil {
	//	m.memory = memoryInfo.RSS
	//} else {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	m.memory = memStats.StackInuse + memStats.HeapInuse + memStats.HeapIdle - memStats.HeapReleased

	return &Snapshot{
		UploadTotal:   m.uploadTotal.Load(),
		DownloadTotal: m.downloadTotal.Load(),
		Connections:   connections,
		Memory:        m.memory,
	}
}

func (m *Manager) ResetStatistic() {
	m.uploadTemp.Store(0)
	m.uploadBlip.Store(0)
	m.uploadTotal.Store(0)
	m.downloadTemp.Store(0)
	m.downloadBlip.Store(0)
	m.downloadTotal.Store(0)
}

func (m *Manager) handle() {
	var uploadTemp int64
	var downloadTemp int64
	for {
		select {
		case <-m.done:
			return
		case <-m.ticker.C:
		}
		uploadTemp = m.uploadTemp.Swap(0)
		downloadTemp = m.downloadTemp.Swap(0)
		m.uploadBlip.Store(uploadTemp)
		m.downloadBlip.Store(downloadTemp)
	}
}

func (m *Manager) Close() error {
	m.ticker.Stop()
	close(m.done)
	return nil
}

type Snapshot struct {
	DownloadTotal int64     `json:"downloadTotal"`
	UploadTotal   int64     `json:"uploadTotal"`
	Connections   []tracker `json:"connections"`
	Memory        uint64    `json:"memory"`
}
