package inbound

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/udpnat"

	"golang.org/x/net/dns/dnsmessage"
)

type DNS struct {
	myInboundAdapter
	udpNat *udpnat.Service[netip.AddrPort]
}

func NewDNS(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.DNSInboundOptions) *DNS {
	dns := &DNS{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeTProxy,
			network:       options.Network.Build(),
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
	}
	dns.connHandler = dns
	dns.packetHandler = dns
	dns.udpNat = udpnat.New[netip.AddrPort](10, adapter.NewUpstreamContextHandler(nil, dns.newPacketConnection, dns))
	dns.packetUpstream = dns.udpNat
	return dns
}

func (d *DNS) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewDNSConnection(ctx, d.router, d.logger, conn, metadata)
}

func (d *DNS) NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, metadata adapter.InboundContext) error {
	d.udpNat.NewContextPacket(ctx, metadata.Source.AddrPort(), buffer, adapter.UpstreamMetadata(metadata), func(natConn N.PacketConn) (context.Context, N.PacketWriter) {
		return adapter.WithContext(log.ContextWithNewID(ctx), &metadata), natConn
	})
	return nil
}

func (d *DNS) newPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return NewDNSPacketConnection(ctx, d.router, d.logger, conn, metadata)
}

func NewDNSConnection(ctx context.Context, router adapter.Router, logger log.ContextLogger, conn net.Conn, metadata adapter.InboundContext) error {
	ctx = adapter.WithContext(ctx, &metadata)
	_buffer := buf.StackNewSize(1024)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	for {
		var queryLength uint16
		err := binary.Read(conn, binary.BigEndian, &queryLength)
		if err != nil {
			return err
		}
		if queryLength > 1024 {
			return io.ErrShortBuffer
		}
		buffer.FullReset()
		_, err = buffer.ReadFullFrom(conn, int(queryLength))
		if err != nil {
			return err
		}
		var message dnsmessage.Message
		err = message.Unpack(buffer.Bytes())
		if err != nil {
			return err
		}
		if len(message.Questions) > 0 {
			question := message.Questions[0]
			metadata.Domain = string(question.Name.Data[:question.Name.Length-1])
			logger.DebugContext(ctx, "inbound dns query ", formatDNSQuestion(question), " from ", metadata.Source)
		}
		go func() error {
			response, err := router.Exchange(ctx, &message)
			if err != nil {
				return err
			}
			_responseBuffer := buf.StackNewSize(1024)
			defer common.KeepAlive(_responseBuffer)
			responseBuffer := common.Dup(_responseBuffer)
			defer responseBuffer.Release()
			responseBuffer.Resize(2, 0)
			n, err := response.AppendPack(responseBuffer.Index(0))
			if err != nil {
				return err
			}
			responseBuffer.Truncate(len(n))
			binary.BigEndian.PutUint16(responseBuffer.ExtendHeader(2), uint16(len(n)))
			_, err = conn.Write(responseBuffer.Bytes())
			return err
		}()
	}
}

func NewDNSPacketConnection(ctx context.Context, router adapter.Router, logger log.ContextLogger, conn N.PacketConn, metadata adapter.InboundContext) error {
	ctx = adapter.WithContext(ctx, &metadata)
	_buffer := buf.StackNewSize(1024)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	for {
		buffer.FullReset()
		destination, err := conn.ReadPacket(buffer)
		if err != nil {
			return err
		}
		var message dnsmessage.Message
		err = message.Unpack(buffer.Bytes())
		if err != nil {
			return err
		}
		if len(message.Questions) > 0 {
			question := message.Questions[0]
			metadata.Domain = string(question.Name.Data[:question.Name.Length-1])
			logger.DebugContext(ctx, "inbound dns query ", formatDNSQuestion(question), " from ", metadata.Source)
		}
		go func() error {
			response, err := router.Exchange(ctx, &message)
			if err != nil {
				return err
			}
			_responseBuffer := buf.StackNewSize(1024)
			defer common.KeepAlive(_responseBuffer)
			responseBuffer := common.Dup(_responseBuffer)
			defer responseBuffer.Release()
			n, err := response.AppendPack(responseBuffer.Index(0))
			if err != nil {
				return err
			}
			responseBuffer.Truncate(len(n))
			err = conn.WritePacket(responseBuffer, destination)
			return err
		}()
	}
}

func formatDNSQuestion(question dnsmessage.Question) string {
	return string(question.Name.Data[:question.Name.Length-1]) + " " + question.Type.String()[4:] + " " + question.Class.String()[5:]
}
