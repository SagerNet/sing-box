package tuic

import (
	"context"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/sing-box/transport/tuic/congestion"
	"github.com/sagernet/sing/common/ntp"
)

func setCongestion(ctx context.Context, connection quic.Connection, congestionName string) {
	timeFunc := ntp.TimeFuncFromContext(ctx)
	if timeFunc == nil {
		timeFunc = time.Now
	}
	switch congestionName {
	case "cubic":
		connection.SetCongestionControl(
			congestion.NewCubicSender(
				congestion.DefaultClock{TimeFunc: timeFunc},
				congestion.GetInitialPacketSize(connection.RemoteAddr()),
				false,
				nil,
			),
		)
	case "new_reno":
		connection.SetCongestionControl(
			congestion.NewCubicSender(
				congestion.DefaultClock{TimeFunc: timeFunc},
				congestion.GetInitialPacketSize(connection.RemoteAddr()),
				true,
				nil,
			),
		)
	case "bbr":
		connection.SetCongestionControl(
			congestion.NewBBRSender(
				congestion.DefaultClock{},
				congestion.GetInitialPacketSize(connection.RemoteAddr()),
				congestion.InitialCongestionWindow*congestion.InitialMaxDatagramSize,
				congestion.DefaultBBRMaxCongestionWindow*congestion.InitialMaxDatagramSize,
			),
		)
	}
}
