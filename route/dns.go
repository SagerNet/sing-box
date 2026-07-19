package route

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	dnsOutbound "github.com/sagernet/sing-box/protocol/dns"
	R "github.com/sagernet/sing-box/route/rule"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

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

func (r *Router) hijackDNSPacket(ctx context.Context, conn N.PacketConn, packetBuffers []*N.PacketBuffer, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) error {
	err := dnsOutbound.NewDNSPacketConnection(ctx, r.dns, conn, packetBuffers, metadata)
	N.CloseOnHandshakeFailure(conn, onClose, err)
	if err != nil && !E.IsClosedOrCanceled(err) {
		return E.Cause(err, "process DNS packet")
	}
	return nil
}

func (r *Router) HijackDNSPacket(ctx context.Context, payload []byte, writer N.PacketWriter, metadata adapter.InboundContext) {
	var message mDNS.Msg
	err := message.Unpack(payload)
	if err != nil {
		r.logger.ErrorContext(ctx, E.Cause(err, "process DNS packet: unpack request"))
		return
	}
	destination := metadata.Destination
	metadata.Destination = M.Socksaddr{}
	r.dns.ExchangeAsync(adapter.WithContext(ctx, &metadata), &message, adapter.DNSQueryOptions{}, func(response *mDNS.Msg, exchangeErr error) {
		if exchangeErr == nil {
			exchangeErr = r.writeDNSPacketResponse(&message, response, writer, destination)
		}
		if exchangeErr != nil && !R.IsRejected(exchangeErr) && !E.IsClosedOrCanceled(exchangeErr) {
			r.logger.ErrorContext(ctx, E.Cause(exchangeErr, "process DNS packet"))
		}
	})
}

func (r *Router) writeDNSPacketResponse(message *mDNS.Msg, response *mDNS.Msg, writer N.PacketWriter, destination M.Socksaddr) error {
	responseBuffer, err := dns.TruncateDNSMessage(message, response, 1024)
	if err != nil {
		return err
	}
	return writer.WritePacket(responseBuffer, destination)
}
