package route

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	dnsOutbound "github.com/sagernet/sing-box/protocol/dns"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/udpnat2"

	mDNS "github.com/miekg/dns"
)

func (r *Router) hijackDNSStream(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	metadata.Destination = M.Socksaddr{}
	for {
		conn.SetReadDeadline(time.Now().Add(C.DNSTimeout))
		err := dnsOutbound.HandleStreamDNSRequest(ctx, r, conn, metadata)
		if err != nil {
			return err
		}
	}
}

func (r *Router) hijackDNSPacket(ctx context.Context, conn N.PacketConn, packetBuffers []*N.PacketBuffer, metadata adapter.InboundContext) {
	if uConn, isUDPNAT2 := conn.(*udpnat.Conn); isUDPNAT2 {
		metadata.Destination = M.Socksaddr{}
		for _, packet := range packetBuffers {
			buffer := packet.Buffer
			destination := packet.Destination
			N.PutPacketBuffer(packet)
			go ExchangeDNSPacket(ctx, r, uConn, buffer, metadata, destination)
		}
		uConn.SetHandler(&dnsHijacker{
			router:   r,
			conn:     conn,
			ctx:      ctx,
			metadata: metadata,
		})
		return
	}
	err := dnsOutbound.NewDNSPacketConnection(ctx, r, conn, packetBuffers, metadata)
	if err != nil && !E.IsClosedOrCanceled(err) {
		r.dnsLogger.ErrorContext(ctx, E.Cause(err, "process packet connection"))
	}
}

func ExchangeDNSPacket(ctx context.Context, router *Router, conn N.PacketConn, buffer *buf.Buffer, metadata adapter.InboundContext, destination M.Socksaddr) {
	err := exchangeDNSPacket(ctx, router, conn, buffer, metadata, destination)
	if err != nil && !errors.Is(err, tun.ErrDrop) && !E.IsClosedOrCanceled(err) {
		router.dnsLogger.ErrorContext(ctx, E.Cause(err, "process packet connection"))
	}
}

func exchangeDNSPacket(ctx context.Context, router *Router, conn N.PacketConn, buffer *buf.Buffer, metadata adapter.InboundContext, destination M.Socksaddr) error {
	var message mDNS.Msg
	err := message.Unpack(buffer.Bytes())
	buffer.Release()
	if err != nil {
		return E.Cause(err, "unpack request")
	}
	response, err := router.Exchange(adapter.WithContext(ctx, &metadata), &message)
	if err != nil {
		return err
	}
	responseBuffer, err := dns.TruncateDNSMessage(&message, response, 1024)
	if err != nil {
		return err
	}
	err = conn.WritePacket(responseBuffer, destination)
	return err
}

type dnsHijacker struct {
	router   *Router
	conn     N.PacketConn
	ctx      context.Context
	metadata adapter.InboundContext
}

func (h *dnsHijacker) NewPacketEx(buffer *buf.Buffer, destination M.Socksaddr) {
	go ExchangeDNSPacket(h.ctx, h.router, h.conn, buffer, h.metadata, destination)
}
