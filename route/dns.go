package route

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	dnsOutbound "github.com/sagernet/sing-box/protocol/dns"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/udpnat2"

	mDNS "github.com/miekg/dns"
)

func (r *Router) hijackDNSStream(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	metadata.Destination = M.Socksaddr{}
	for {
		conn.SetReadDeadline(time.Now().Add(C.DNSTimeout))
		err := dnsOutbound.HandleStreamDNSRequest(ctx, r.dns, conn, metadata)
		if err != nil {
			if !E.IsClosedOrCanceled(err) {
				return err
			} else {
				return nil
			}
		}
	}
}

func (r *Router) hijackDNSPacket(ctx context.Context, conn N.PacketConn, packetBuffers []*N.PacketBuffer, metadata adapter.InboundContext) error {
	if natConn, isNatConn := conn.(udpnat.Conn); isNatConn {
		metadata.Destination = M.Socksaddr{}
		for _, packet := range packetBuffers {
			buffer := packet.Buffer
			destination := packet.Destination
			N.PutPacketBuffer(packet)
			go ExchangeDNSPacket(ctx, r.dns, r.logger, natConn, buffer, metadata, destination)
		}
		natConn.SetHandler(&dnsHijacker{
			router:   r.dns,
			logger:   r.logger,
			conn:     conn,
			ctx:      ctx,
			metadata: metadata,
		})
		return nil
	}
	err := dnsOutbound.NewDNSPacketConnection(ctx, r.dns, conn, packetBuffers, metadata)
	if err != nil && !E.IsClosedOrCanceled(err) {
		return E.Cause(err, "process DNS packet")
	}
	return nil
}

func ExchangeDNSPacket(ctx context.Context, router adapter.DNSRouter, logger logger.ContextLogger, conn N.PacketConn, buffer *buf.Buffer, metadata adapter.InboundContext, destination M.Socksaddr) {
	err := exchangeDNSPacket(ctx, router, conn, buffer, metadata, destination)
	if err != nil && !errors.Is(err, tun.ErrDrop) && !E.IsClosedOrCanceled(err) {
		logger.ErrorContext(ctx, E.Cause(err, "process DNS packet"))
	}
}

func exchangeDNSPacket(ctx context.Context, router adapter.DNSRouter, conn N.PacketConn, buffer *buf.Buffer, metadata adapter.InboundContext, destination M.Socksaddr) error {
	var message mDNS.Msg
	err := message.Unpack(buffer.Bytes())
	buffer.Release()
	if err != nil {
		return E.Cause(err, "unpack request")
	}
	response, err := router.Exchange(adapter.WithContext(ctx, &metadata), &message, adapter.DNSQueryOptions{})
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
	router   adapter.DNSRouter
	logger   logger.ContextLogger
	conn     N.PacketConn
	ctx      context.Context
	metadata adapter.InboundContext
}

func (h *dnsHijacker) NewPacketEx(buffer *buf.Buffer, destination M.Socksaddr) {
	go ExchangeDNSPacket(h.ctx, h.router, h.logger, h.conn, buffer, h.metadata, destination)
}
