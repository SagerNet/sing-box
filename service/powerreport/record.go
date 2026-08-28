package powerreport

type Direction uint8

const (
	DirectionOutbound Direction = iota
	DirectionInbound
)

func (d Direction) String() string {
	if d == DirectionInbound {
		return "in"
	}
	return "out"
}

type Attribution struct {
	Inbound      string              `json:"inbound,omitempty"`
	InboundType  string              `json:"inboundType,omitempty"`
	Network      string              `json:"network,omitempty"`
	Source       string              `json:"source,omitempty"`
	Destination  string              `json:"destination,omitempty"`
	Domain       string              `json:"domain,omitempty"`
	Protocol     string              `json:"protocol,omitempty"`
	User         string              `json:"user,omitempty"`
	Process      *ProcessAttribution `json:"process,omitempty"`
	Rule         string              `json:"rule,omitempty"`
	Chain        []string            `json:"chain,omitempty"`
	Outbound     string              `json:"outbound,omitempty"`
	OutboundType string              `json:"outboundType,omitempty"`
	Server       string              `json:"server,omitempty"`
	DNS          string              `json:"dns,omitempty"`
	DNSType      string              `json:"dnsType,omitempty"`
	Endpoint     string              `json:"endpoint,omitempty"`
}

type ProcessAttribution struct {
	ProcessID    uint32   `json:"processId,omitempty"`
	UserID       int32    `json:"userId,omitempty"`
	UserName     string   `json:"userName,omitempty"`
	ProcessPath  string   `json:"processPath,omitempty"`
	PackageNames []string `json:"packageNames,omitempty"`
}

type timelineRow struct {
	From                        string            `json:"from"`
	To                          string            `json:"to"`
	CPUUserMS                   int64             `json:"cpuUserMS,omitempty"`
	CPUSystemMS                 int64             `json:"cpuSystemMS,omitempty"`
	CPUPerformanceMS            int64             `json:"cpuPerformanceMS,omitempty"`
	CPUGCMS                     int64             `json:"cpuGCMS,omitempty"`
	QoSMS                       *qosBreakdown     `json:"qosMS,omitempty"`
	PackageIdleWakeups          uint64            `json:"packageIdleWakeups,omitempty"`
	InterruptWakeups            uint64            `json:"interruptWakeups,omitempty"`
	EnergyNanojoules            uint64            `json:"energyNJ,omitempty"`
	PerformanceEnergyNanojoules uint64            `json:"performanceEnergyNJ,omitempty"`
	DiskBytesWritten            uint64            `json:"diskWriteBytes,omitempty"`
	SleptMS                     int64             `json:"sleptMS,omitempty"`
	Goroutines                  uint64            `json:"goroutines,omitempty"`
	GCCycles                    uint64            `json:"gcCycles,omitempty"`
	GoMemoryBytes               uint64            `json:"goMemoryBytes,omitempty"`
	GoHeapLiveBytes             uint64            `json:"goHeapLiveBytes,omitempty"`
	MemoryBytes                 uint64            `json:"memoryBytes,omitempty"`
	DNSQueries                  uint64            `json:"dnsQueries,omitempty"`
	ConnectionsOpened           uint64            `json:"connectionsOpened,omitempty"`
	InterfacePackets            map[string]uint64 `json:"interfacePackets,omitempty"`
	NetworkType                 string            `json:"network,omitempty"`
}

type qosBreakdown struct {
	DefaultMS         int64 `json:"default,omitempty"`
	MaintenanceMS     int64 `json:"maintenance,omitempty"`
	BackgroundMS      int64 `json:"background,omitempty"`
	UtilityMS         int64 `json:"utility,omitempty"`
	LegacyMS          int64 `json:"legacy,omitempty"`
	UserInitiatedMS   int64 `json:"userInitiated,omitempty"`
	UserInteractiveMS int64 `json:"userInteractive,omitempty"`
}

const (
	eventTypeBreak   = "break"
	eventTypeNetwork = "network"
)

type eventRecord struct {
	Type        string       `json:"t"`
	At          string       `json:"at"`
	IdleMS      int64        `json:"idleMS,omitempty"`
	Direction   string       `json:"direction,omitempty"`
	Size        int          `json:"size,omitempty"`
	NetworkType string       `json:"network,omitempty"`
	By          *Attribution `json:"by,omitempty"`
}

type systemUsage struct {
	valid                       bool
	userTime                    int64
	systemTime                  int64
	performanceUserTime         int64
	performanceSystemTime       int64
	qosDefaultTime              int64
	qosMaintenanceTime          int64
	qosBackgroundTime           int64
	qosUtilityTime              int64
	qosLegacyTime               int64
	qosUserInitiatedTime        int64
	qosUserInteractiveTime      int64
	packageIdleWakeups          uint64
	interruptWakeups            uint64
	diskBytesWritten            uint64
	energyNanojoules            uint64
	performanceEnergyNanojoules uint64
}

type interfaceCounters struct {
	inPackets  uint32
	outPackets uint32
}
