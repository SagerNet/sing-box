package libbox

import "github.com/sagernet/sing-box/daemon"

type STUNTestProgress struct {
	Phase        int32
	ExternalAddr string
	LatencyMs    int32
	NATMapping   int32
	NATFiltering int32
}

type STUNTestResult struct {
	ExternalAddr     string
	LatencyMs        int32
	NATMapping       int32
	NATFiltering     int32
	NATTypeSupported bool
}

type STUNTestHandler interface {
	OnProgress(progress *STUNTestProgress)
	OnResult(result *STUNTestResult)
	OnError(message string)
}

func stunTestProgressFromGRPC(event *daemon.STUNTestProgress) *STUNTestProgress {
	return &STUNTestProgress{
		Phase:        event.Phase,
		ExternalAddr: event.ExternalAddr,
		LatencyMs:    event.LatencyMs,
		NATMapping:   event.NatMapping,
		NATFiltering: event.NatFiltering,
	}
}
