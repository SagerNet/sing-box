package dns

import (
	"context"
	"encoding/binary"
	"net"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/canceler"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/task"

	mDNS "github.com/miekg/dns"
)

func HandleStreamDNSRequest(ctx context.Context, router adapter.DNSRouter, conn net.Conn, metadata adapter.InboundContext) error {
	var queryLength uint16
	err := binary.Read(conn, binary.BigEndian, &queryLength)
	if err != nil {
		return err
	}
	if queryLength == 0 {
		return dns.RcodeFormatError
	}
	buffer := buf.NewSize(int(queryLength))
	defer buffer.Release()
	_, err = buffer.ReadFullFrom(conn, int(queryLength))
	if err != nil {
		return err
	}
	var message mDNS.Msg
	err = message.Unpack(buffer.Bytes())
	if err != nil {
		return err
	}
	metadataInQuery := metadata
	go func() error {
		response, err := router.Exchange(adapter.WithContext(ctx, &metadataInQuery), &message, adapter.DNSQueryOptions{})
		if err != nil {
			conn.Close()
			return err
		}
		responseBuffer := buf.NewPacket()
		defer responseBuffer.Release()
		responseBuffer.Resize(2, 0)
		n, err := response.PackBuffer(responseBuffer.FreeBytes())
		if err != nil {
			return err
		}
		responseBuffer.Truncate(len(n))
		binary.BigEndian.PutUint16(responseBuffer.ExtendHeader(2), uint16(len(n)))
		_, err = conn.Write(responseBuffer.Bytes())
		return err
	}()
	return nil
}

func NewDNSPacketConnection(ctx context.Context, router adapter.DNSRouter, conn N.PacketConn, cachedPackets []*N.PacketBuffer, metadata adapter.InboundContext) error {
	metadata.Destination = M.Socksaddr{}
	var reader N.PacketReader = conn
	var counters []N.CountFunc
	cachedPackets = common.Reverse(cachedPackets)
	for {
		reader, counters = N.UnwrapCountPacketReader(reader, counters)
		if cachedReader, isCached := reader.(N.CachedPacketReader); isCached {
			packet := cachedReader.ReadCachedPacket()
			if packet != nil {
				cachedPackets = append(cachedPackets, packet)
				continue
			}
		}
		if readWaiter, created := bufio.CreatePacketReadWaiter(reader); created {
			readWaiter.InitializeReadWaiter(N.ReadWaitOptions{})
			return newDNSPacketConnection(ctx, router, conn, readWaiter, counters, cachedPackets, metadata)
		}
		break
	}
	fastClose, cancel := common.ContextWithCancelCause(ctx)
	timeout := canceler.New(fastClose, cancel, C.DNSTimeout)
	var group task.Group
	group.Append0(func(_ context.Context) error {
		for {
			var message mDNS.Msg
			var destination M.Socksaddr
			var err error
			if len(cachedPackets) > 0 {
				packet := cachedPackets[0]
				cachedPackets = cachedPackets[1:]
				for _, counter := range counters {
					counter(int64(packet.Buffer.Len()))
				}
				err = message.Unpack(packet.Buffer.Bytes())
				packet.Buffer.Release()
				if err != nil {
					cancel(err)
					return err
				}
				destination = packet.Destination
			} else {
				buffer := buf.NewPacket()
				destination, err = conn.ReadPacket(buffer)
				if err != nil {
					buffer.Release()
					cancel(err)
					return err
				}
				for _, counter := range counters {
					counter(int64(buffer.Len()))
				}
				err = message.Unpack(buffer.Bytes())
				buffer.Release()
				if err != nil {
					cancel(err)
					return err
				}
				timeout.Update()
			}
			metadataInQuery := metadata
			go func() error {
				response, err := router.Exchange(adapter.WithContext(ctx, &metadataInQuery), &message, adapter.DNSQueryOptions{})
				if err != nil {
					cancel(err)
					return err
				}
				timeout.Update()
				responseBuffer, err := dns.TruncateDNSMessage(&message, response, 1024)
				if err != nil {
					cancel(err)
					return err
				}
				err = conn.WritePacket(responseBuffer, destination)
				if err != nil {
					cancel(err)
				}
				return err
			}()
		}
	})
	group.Cleanup(func() {
		conn.Close()
	})
	return group.Run(fastClose)
}

func newDNSPacketConnection(ctx context.Context, router adapter.DNSRouter, conn N.PacketConn, readWaiter N.PacketReadWaiter, readCounters []N.CountFunc, cached []*N.PacketBuffer, metadata adapter.InboundContext) error {
	fastClose, cancel := common.ContextWithCancelCause(ctx)
	timeout := canceler.New(fastClose, cancel, C.DNSTimeout)
	var group task.Group
	group.Append0(func(_ context.Context) error {
		for {
			var (
				message     mDNS.Msg
				destination M.Socksaddr
				err         error
				buffer      *buf.Buffer
			)
			if len(cached) > 0 {
				packet := cached[0]
				cached = cached[1:]
				for _, counter := range readCounters {
					counter(int64(packet.Buffer.Len()))
				}
				err = message.Unpack(packet.Buffer.Bytes())
				packet.Buffer.Release()
				destination = packet.Destination
				N.PutPacketBuffer(packet)
				if err != nil {
					cancel(err)
					return err
				}
			} else {
				buffer, destination, err = readWaiter.WaitReadPacket()
				if err != nil {
					cancel(err)
					return err
				}
				for _, counter := range readCounters {
					counter(int64(buffer.Len()))
				}
				err = message.Unpack(buffer.Bytes())
				buffer.Release()
				if err != nil {
					cancel(err)
					return err
				}
				timeout.Update()
			}
			metadataInQuery := metadata
			go func() error {
				response, err := router.Exchange(adapter.WithContext(ctx, &metadataInQuery), &message, adapter.DNSQueryOptions{})
				if err != nil {
					cancel(err)
					return err
				}
				timeout.Update()
				responseBuffer, err := dns.TruncateDNSMessage(&message, response, 1024)
				if err != nil {
					cancel(err)
					return err
				}
				err = conn.WritePacket(responseBuffer, destination)
				if err != nil {
					cancel(err)
				}
				return err
			}()
		}
	})
	group.Cleanup(func() {
		conn.Close()
	})
	return group.Run(fastClose)
}
