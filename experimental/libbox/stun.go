package libbox

import (
	"context"

	"github.com/sagernet/sing-box/common/stun"
)

type STUNTest struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func NewSTUNTest() *STUNTest {
	ctx, cancel := context.WithCancel(context.Background())
	return &STUNTest{ctx: ctx, cancel: cancel}
}

func (t *STUNTest) Start(server string, handler STUNTestHandler) {
	go func() {
		result, err := stun.Run(stun.Options{
			Server:  server,
			Context: t.ctx,
			OnProgress: func(p stun.Progress) {
				handler.OnProgress(&STUNTestProgress{
					Phase:        int32(p.Phase),
					ExternalAddr: p.ExternalAddr,
					LatencyMs:    p.LatencyMs,
					NATMapping:   int32(p.NATMapping),
					NATFiltering: int32(p.NATFiltering),
				})
			},
		})
		if err != nil {
			handler.OnError(err.Error())
			return
		}
		handler.OnResult(&STUNTestResult{
			ExternalAddr:     result.ExternalAddr,
			LatencyMs:        result.LatencyMs,
			NATMapping:       int32(result.NATMapping),
			NATFiltering:     int32(result.NATFiltering),
			NATTypeSupported: result.NATTypeSupported,
		})
	}()
}

func (t *STUNTest) Cancel() {
	t.cancel()
}
