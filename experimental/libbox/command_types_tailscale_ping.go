package libbox

import "github.com/sagernet/sing-box/daemon"

type TailscalePingResult struct {
	LatencyMs      float64
	IsDirect       bool
	Endpoint       string
	DERPRegionID   int32
	DERPRegionCode string
	Error          string
}

type TailscalePingHandler interface {
	OnPingResult(result *TailscalePingResult)
	OnError(message string)
}

func tailscalePingResultFromGRPC(response *daemon.TailscalePingResponse) *TailscalePingResult {
	return &TailscalePingResult{
		LatencyMs:      response.LatencyMs,
		IsDirect:       response.IsDirect,
		Endpoint:       response.Endpoint,
		DERPRegionID:   response.DerpRegionID,
		DERPRegionCode: response.DerpRegionCode,
		Error:          response.Error,
	}
}
