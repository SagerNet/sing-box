package adapter

import (
	"context"
	"net/netip"
	"sync/atomic"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-tun/gtcpip/header"
	"github.com/sagernet/sing/common/byteformats"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func JudgeHostFlow(ctx context.Context, networkManager NetworkManager, logger log.ContextLogger, network uint8, source netip.AddrPort, destination netip.AddrPort) tun.FlowVerdict {
	port := networkManager.HostPort()
	if port == nil {
		return tun.FlowVerdict{Action: tun.ActionAccept}
	}
	inet4Address, inet6Address := port.PortAddresses()
	hostAddress := inet4Address
	if destination.Addr().Is6() {
		hostAddress = inet6Address
	}
	networkName := flowNetworkName(network)
	if !hostAddress.IsValid() || networkName == "" {
		return tun.FlowVerdict{Action: tun.ActionAccept}
	}
	hostDestination := netip.AddrPortFrom(hostAddress, destination.Port())
	return tun.FlowVerdict{
		Action:      tun.ActionFlow,
		Port:        port,
		Destination: hostDestination,
		NewTracker: func() tun.FlowTracker {
			flowContext := log.ContextWithNewID(ctx)
			sourceAddress := M.SocksaddrFromNetIP(source)
			destinationAddress := M.SocksaddrFromNetIP(hostDestination)
			if networkName == N.NetworkICMP {
				sourceAddress.Port = 0
				destinationAddress.Port = 0
			}
			logger.InfoContext(flowContext, "forward ", networkName, " connection from ", sourceAddress, " to ", destinationAddress, " via tun")
			return NewFlowLogger(flowContext, logger)
		},
	}
}

func flowNetworkName(network uint8) string {
	switch network {
	case uint8(header.TCPProtocolNumber):
		return N.NetworkTCP
	case uint8(header.UDPProtocolNumber):
		return N.NetworkUDP
	case uint8(header.ICMPv4ProtocolNumber), uint8(header.ICMPv6ProtocolNumber):
		return N.NetworkICMP
	default:
		return ""
	}
}

var _ tun.FlowTracker = (*FlowLogger)(nil)

type FlowLogger struct {
	ctx      context.Context
	logger   log.ContextLogger
	upload   atomic.Int64
	download atomic.Int64
}

func NewFlowLogger(ctx context.Context, logger log.ContextLogger) *FlowLogger {
	return &FlowLogger{ctx: ctx, logger: logger}
}

func (l *FlowLogger) AttachFlow(handle tun.FlowHandle) {
}

func (l *FlowLogger) CountForward(n int) {
	l.upload.Add(int64(n))
}

func (l *FlowLogger) CountReverse(n int) {
	l.download.Add(int64(n))
}

func (l *FlowLogger) FlowEstablished() {
}

func (l *FlowLogger) CloseFlow(reason tun.FlowCloseReason) {
	l.logger.DebugContext(l.ctx, "flow closed: ", reason,
		", upload ", byteformats.FormatBytes(uint64(l.upload.Load())), ", download ", byteformats.FormatBytes(uint64(l.download.Load())))
}
