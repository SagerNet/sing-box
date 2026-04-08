package libbox

import "github.com/sagernet/sing-box/daemon"

type NetworkQualityProgress struct {
	Phase                    int32
	DownloadCapacity         int64
	UploadCapacity           int64
	DownloadRPM              int32
	UploadRPM                int32
	IdleLatencyMs            int32
	ElapsedMs                int64
	DownloadCapacityAccuracy int32
	UploadCapacityAccuracy   int32
	DownloadRPMAccuracy      int32
	UploadRPMAccuracy        int32
}

type NetworkQualityResult struct {
	DownloadCapacity         int64
	UploadCapacity           int64
	DownloadRPM              int32
	UploadRPM                int32
	IdleLatencyMs            int32
	DownloadCapacityAccuracy int32
	UploadCapacityAccuracy   int32
	DownloadRPMAccuracy      int32
	UploadRPMAccuracy        int32
}

type NetworkQualityTestHandler interface {
	OnProgress(progress *NetworkQualityProgress)
	OnResult(result *NetworkQualityResult)
	OnError(message string)
}

func networkQualityProgressFromGRPC(event *daemon.NetworkQualityTestProgress) *NetworkQualityProgress {
	return &NetworkQualityProgress{
		Phase:                    event.Phase,
		DownloadCapacity:         event.DownloadCapacity,
		UploadCapacity:           event.UploadCapacity,
		DownloadRPM:              event.DownloadRPM,
		UploadRPM:                event.UploadRPM,
		IdleLatencyMs:            event.IdleLatencyMs,
		ElapsedMs:                event.ElapsedMs,
		DownloadCapacityAccuracy: event.DownloadCapacityAccuracy,
		UploadCapacityAccuracy:   event.UploadCapacityAccuracy,
		DownloadRPMAccuracy:      event.DownloadRPMAccuracy,
		UploadRPMAccuracy:        event.UploadRPMAccuracy,
	}
}
