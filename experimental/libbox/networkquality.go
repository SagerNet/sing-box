package libbox

import (
	"context"
	"time"

	"github.com/sagernet/sing-box/common/networkquality"
)

type NetworkQualityTest struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func NewNetworkQualityTest() *NetworkQualityTest {
	ctx, cancel := context.WithCancel(context.Background())
	return &NetworkQualityTest{ctx: ctx, cancel: cancel}
}

func (t *NetworkQualityTest) Start(configURL string, serial bool, maxRuntimeSeconds int32, http3 bool, handler NetworkQualityTestHandler) {
	go func() {
		httpClient := networkquality.NewHTTPClient(nil)
		defer httpClient.CloseIdleConnections()

		measurementClientFactory, err := networkquality.NewOptionalHTTP3Factory(nil, http3)
		if err != nil {
			handler.OnError(err.Error())
			return
		}

		result, err := networkquality.Run(networkquality.Options{
			ConfigURL:            configURL,
			HTTPClient:           httpClient,
			NewMeasurementClient: measurementClientFactory,
			Serial:               serial,
			MaxRuntime:           time.Duration(maxRuntimeSeconds) * time.Second,
			Context:              t.ctx,
			OnProgress: func(p networkquality.Progress) {
				handler.OnProgress(&NetworkQualityProgress{
					Phase:                    int32(p.Phase),
					DownloadCapacity:         p.DownloadCapacity,
					UploadCapacity:           p.UploadCapacity,
					DownloadRPM:              p.DownloadRPM,
					UploadRPM:                p.UploadRPM,
					IdleLatencyMs:            p.IdleLatencyMs,
					ElapsedMs:                p.ElapsedMs,
					DownloadCapacityAccuracy: int32(p.DownloadCapacityAccuracy),
					UploadCapacityAccuracy:   int32(p.UploadCapacityAccuracy),
					DownloadRPMAccuracy:      int32(p.DownloadRPMAccuracy),
					UploadRPMAccuracy:        int32(p.UploadRPMAccuracy),
				})
			},
		})
		if err != nil {
			handler.OnError(err.Error())
			return
		}
		handler.OnResult(&NetworkQualityResult{
			DownloadCapacity:         result.DownloadCapacity,
			UploadCapacity:           result.UploadCapacity,
			DownloadRPM:              result.DownloadRPM,
			UploadRPM:                result.UploadRPM,
			IdleLatencyMs:            result.IdleLatencyMs,
			DownloadCapacityAccuracy: int32(result.DownloadCapacityAccuracy),
			UploadCapacityAccuracy:   int32(result.UploadCapacityAccuracy),
			DownloadRPMAccuracy:      int32(result.DownloadRPMAccuracy),
			UploadRPMAccuracy:        int32(result.UploadRPMAccuracy),
		})
	}()
}

func (t *NetworkQualityTest) Cancel() {
	t.cancel()
}
