//go:build with_tailscale

package tailscale

import (
	"context"
	"net"
	"net/netip"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/tailscale/net/netmon"
	"github.com/sagernet/tailscale/types/nettype"
)

type systemBinding struct {
	control      func(network, address string, conn syscall.RawConn) error
	listenPacket func(ctx context.Context, network, address string) (nettype.PacketConn, error)
}

func newSystemBinding(ctx context.Context, logger logger.ContextLogger) (systemBinding, error) {
	network := service.FromContext[adapter.NetworkManager](ctx)
	platformInterface := service.FromContext[adapter.PlatformInterface](ctx)
	if platformInterface != nil && platformInterface.UsePlatformNetworkInterfaces() {
		err := network.UpdateInterfaces()
		if err != nil {
			return systemBinding{}, err
		}
		netmon.RegisterInterfaceGetter(func() ([]netmon.Interface, error) {
			return common.Map(network.InterfaceFinder().Interfaces(), func(it control.Interface) netmon.Interface {
				return netmon.Interface{
					Interface: &net.Interface{
						Index:        it.Index,
						MTU:          it.MTU,
						Name:         it.Name,
						HardwareAddr: it.HardwareAddr,
						Flags:        it.Flags,
					},
					AltAddrs: common.Map(it.Addresses, func(it netip.Prefix) net.Addr {
						return &net.IPNet{
							IP:   it.Addr().AsSlice(),
							Mask: net.CIDRMask(it.Bits(), it.Addr().BitLen()),
						}
					}),
				}
			}), nil
		})
	}
	if network.AutoRedirectOutputMark() != 0 {
		return systemBinding{control: network.AutoRedirectOutputMarkFunc()}, nil
	}
	if platformInterface != nil && platformInterface.UsePlatformNetworkInterfaces() {
		if platformInterface.UsePlatformAutoDetectInterfaceControl() {
			return systemBinding{control: func(network, address string, conn syscall.RawConn) error {
				return control.Raw(conn, func(fileDescriptor uintptr) error {
					return platformInterface.AutoDetectInterfaceControl(int(fileDescriptor))
				})
			}}, nil
		}
		// NEPacketTunnelProvider sockets are excluded from tunnel routes by
		// NECP; the empty override only suppresses tailscale's own
		// default-interface bind, which would select the sing-box utun.
		return systemBinding{control: func(string, string, syscall.RawConn) error {
			return nil
		}}, nil
	}
	bindFunc := network.AutoDetectInterfaceFunc()
	if bindFunc == nil {
		return systemBinding{}, nil
	}
	return systemBinding{
		control: bindFunc,
		listenPacket: func(ctx context.Context, networkName string, address string) (nettype.PacketConn, error) {
			listenConfig := net.ListenConfig{
				Control: control.Append(bindFunc, control.DisableUDPNetReset()),
			}
			packetConn, err := listenConfig.ListenPacket(ctx, networkName, address)
			if err != nil {
				return nil, err
			}
			udpConn := packetConn.(*net.UDPConn)
			egressPool := tun.NewUDPEgressPool(tun.UDPEgressPoolOptions{
				Logger:           logger,
				Network:          networkName,
				InterfaceFinder:  network.InterfaceFinder(),
				InterfaceMonitor: network.InterfaceMonitor(),
				IsExempt: func() bool {
					return network.AutoRedirectOutputMark() != 0
				},
			})
			if !egressPool.SetEgressPort(udpConn.LocalAddr().(*net.UDPAddr).AddrPort().Port()) {
				egressPool.Close()
				return udpConn, nil
			}
			return tun.NewUDPEgressConn(udpConn, egressPool), nil
		},
	}, nil
}

func (b systemBinding) hooks(dialer N.Dialer) netmon.Hooks {
	return netmon.Hooks{Dialer: dialer, Control: b.control, ListenPacket: b.listenPacket}
}
