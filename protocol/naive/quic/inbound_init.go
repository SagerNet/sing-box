package quic

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/congestion"
	"github.com/sagernet/quic-go/http3"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/naive"
	"github.com/sagernet/sing-quic"
	"github.com/sagernet/sing-quic/congestion_bbr1"
	"github.com/sagernet/sing-quic/congestion_bbr2"
	congestion_meta1 "github.com/sagernet/sing-quic/congestion_meta1"
	congestion_meta2 "github.com/sagernet/sing-quic/congestion_meta2"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/ntp"
)

func init() {
	naive.ConfigureHTTP3ListenerFunc = func(ctx context.Context, logger logger.Logger, listener *listener.Listener, handler http.Handler, tlsConfig tls.ServerConfig, options option.NaiveInboundOptions) (io.Closer, error) {
		err := qtls.ConfigureHTTP3(tlsConfig)
		if err != nil {
			return nil, err
		}

		udpConn, err := listener.ListenUDP()
		if err != nil {
			return nil, err
		}

		var congestionControl func(conn *quic.Conn) congestion.CongestionControl
		timeFunc := ntp.TimeFuncFromContext(ctx)
		if timeFunc == nil {
			timeFunc = time.Now
		}
		switch options.QUICCongestionControl {
		case "", "bbr":
			congestionControl = func(conn *quic.Conn) congestion.CongestionControl {
				return congestion_meta2.NewBbrSender(
					congestion_meta2.DefaultClock{TimeFunc: timeFunc},
					congestion.ByteCount(conn.Config().InitialPacketSize),
					congestion.ByteCount(congestion_meta1.InitialCongestionWindow),
				)
			}
		case "bbr_standard":
			congestionControl = func(conn *quic.Conn) congestion.CongestionControl {
				return congestion_bbr1.NewBbrSender(
					congestion_bbr1.DefaultClock{TimeFunc: timeFunc},
					congestion.ByteCount(conn.Config().InitialPacketSize),
					congestion_bbr1.InitialCongestionWindowPackets,
					congestion_bbr1.MaxCongestionWindowPackets,
				)
			}
		case "bbr2":
			congestionControl = func(conn *quic.Conn) congestion.CongestionControl {
				return congestion_bbr2.NewBBR2Sender(
					congestion_bbr2.DefaultClock{TimeFunc: timeFunc},
					congestion.ByteCount(conn.Config().InitialPacketSize),
					0,
					false,
				)
			}
		case "bbr2_variant":
			congestionControl = func(conn *quic.Conn) congestion.CongestionControl {
				return congestion_bbr2.NewBBR2Sender(
					congestion_bbr2.DefaultClock{TimeFunc: timeFunc},
					congestion.ByteCount(conn.Config().InitialPacketSize),
					32*congestion.ByteCount(conn.Config().InitialPacketSize),
					true,
				)
			}
		case "cubic":
			congestionControl = func(conn *quic.Conn) congestion.CongestionControl {
				return congestion_meta1.NewCubicSender(
					congestion_meta1.DefaultClock{TimeFunc: timeFunc},
					congestion.ByteCount(conn.Config().InitialPacketSize),
					false,
				)
			}
		case "reno":
			congestionControl = func(conn *quic.Conn) congestion.CongestionControl {
				return congestion_meta1.NewCubicSender(
					congestion_meta1.DefaultClock{TimeFunc: timeFunc},
					congestion.ByteCount(conn.Config().InitialPacketSize),
					true,
				)
			}
		default:
			return nil, E.New("unknown quic congestion control: ", options.QUICCongestionControl)
		}

		quicListener, err := qtls.ListenEarly(udpConn, tlsConfig, &quic.Config{
			MaxIncomingStreams: 1 << 60,
			Allow0RTT:          true,
		})
		if err != nil {
			udpConn.Close()
			return nil, err
		}

		h3Server := &http3.Server{
			Handler: handler,
			ConnContext: func(ctx context.Context, conn *quic.Conn) context.Context {
				conn.SetCongestionControl(congestionControl(conn))
				return log.ContextWithNewID(ctx)
			},
		}

		go func() {
			sErr := h3Server.ServeListener(quicListener)
			udpConn.Close()
			if sErr != nil && !E.IsClosedOrCanceled(sErr) {
				logger.Error("http3 server closed: ", sErr)
			}
		}()

		return quicListener, nil
	}
}
