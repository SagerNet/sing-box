package dialer

import (
	"context"
	"net"

	"github.com/jobberrt/sing-box/adapter"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func NewResolvePacketConn(ctx context.Context, router adapter.Router, strategy dns.DomainStrategy, conn net.PacketConn) N.NetPacketConn {
	if udpConn, ok := conn.(*net.UDPConn); ok {
		return &ResolveUDPConn{udpConn, ctx, router, strategy}
	} else {
		return &ResolvePacketConn{conn, ctx, router, strategy}
	}
}

type ResolveUDPConn struct {
	*net.UDPConn
	ctx      context.Context
	router   adapter.Router
	strategy dns.DomainStrategy
}

func (w *ResolveUDPConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	n, addr, err := w.ReadFromUDPAddrPort(buffer.FreeBytes())
	if err != nil {
		return M.Socksaddr{}, err
	}
	buffer.Truncate(n)
	return M.SocksaddrFromNetIP(addr), nil
}

func (w *ResolveUDPConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	if destination.IsFqdn() {
		addresses, err := w.router.Lookup(w.ctx, destination.Fqdn, w.strategy)
		if err != nil {
			return err
		}
		return common.Error(w.UDPConn.WriteToUDPAddrPort(buffer.Bytes(), M.SocksaddrFrom(addresses[0], destination.Port).AddrPort()))
	}
	return common.Error(w.UDPConn.WriteToUDPAddrPort(buffer.Bytes(), destination.AddrPort()))
}

func (w *ResolveUDPConn) Upstream() any {
	return w.UDPConn
}

type ResolvePacketConn struct {
	net.PacketConn
	ctx      context.Context
	router   adapter.Router
	strategy dns.DomainStrategy
}

func (w *ResolvePacketConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	_, addr, err := buffer.ReadPacketFrom(w)
	if err != nil {
		return M.Socksaddr{}, err
	}
	return M.SocksaddrFromNet(addr), err
}

func (w *ResolvePacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	if destination.IsFqdn() {
		addresses, err := w.router.Lookup(w.ctx, destination.Fqdn, w.strategy)
		if err != nil {
			return err
		}
		return common.Error(w.WriteTo(buffer.Bytes(), M.SocksaddrFrom(addresses[0], destination.Port).UDPAddr()))
	}
	return common.Error(w.WriteTo(buffer.Bytes(), destination.UDPAddr()))
}

func (w *ResolvePacketConn) Upstream() any {
	return w.PacketConn
}
