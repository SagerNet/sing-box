package outbound

import (
	"context"
	"encoding/binary"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/canceler"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/task"

	mDNS "github.com/miekg/dns"
)

var _ adapter.Outbound = (*DNS)(nil)

type DNS struct {
	myOutboundAdapter
}

func NewDNS(router adapter.Router, tag string) *DNS {
	return &DNS{
		myOutboundAdapter{
			protocol: C.TypeDNS,
			network:  []string{N.NetworkTCP, N.NetworkUDP},
			router:   router,
			tag:      tag,
		},
	}
}

func (d *DNS) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return nil, os.ErrInvalid
}

func (d *DNS) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func (d *DNS) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	defer conn.Close()
	ctx = adapter.WithContext(ctx, &metadata)
	for {
		err := d.handleConnection(ctx, conn, metadata)
		if err != nil {
			return err
		}
	}
}

func (d *DNS) handleConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	var queryLength uint16
	err := binary.Read(conn, binary.BigEndian, &queryLength)
	if err != nil {
		return err
	}
	if queryLength == 0 {
		return dns.RCodeFormatError
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
		response, err := d.router.Exchange(adapter.WithContext(ctx, &metadataInQuery), &message)
		if err != nil {
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

func (d *DNS) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	var reader N.PacketReader = conn
	var counters []N.CountFunc
	var cachedPackets []*N.PacketBuffer
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
			return d.newPacketConnection(ctx, conn, readWaiter, counters, cachedPackets, metadata)
		}
		break
	}
	ctx = adapter.WithContext(ctx, &metadata)
	fastClose, cancel := common.ContextWithCancelCause(ctx)
	timeout := canceler.New(fastClose, cancel, C.DNSTimeout)
	var group task.Group
	group.Append0(func(ctx context.Context) error {
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
				response, err := d.router.Exchange(adapter.WithContext(ctx, &metadataInQuery), &message)
				if err != nil {
					cancel(err)
					return err
				}
				timeout.Update()
				responseBuffer := buf.NewPacket()
				n, err := response.PackBuffer(responseBuffer.FreeBytes())
				if err != nil {
					cancel(err)
					responseBuffer.Release()
					return err
				}
				responseBuffer.Truncate(len(n))
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

func (d *DNS) newPacketConnection(ctx context.Context, conn N.PacketConn, readWaiter N.PacketReadWaiter, readCounters []N.CountFunc, cached []*N.PacketBuffer, metadata adapter.InboundContext) error {
	ctx = adapter.WithContext(ctx, &metadata)
	fastClose, cancel := common.ContextWithCancelCause(ctx)
	timeout := canceler.New(fastClose, cancel, C.DNSTimeout)
	var group task.Group
	group.Append0(func(ctx context.Context) error {
		var buffer *buf.Buffer
		readWaiter.InitializeReadWaiter(func() *buf.Buffer {
			buffer = buf.NewSize(dns.FixedPacketSize)
			buffer.FullReset()
			return buffer
		})
		defer readWaiter.InitializeReadWaiter(nil)
		for {
			var message mDNS.Msg
			var destination M.Socksaddr
			var err error
			if len(cached) > 0 {
				packet := cached[0]
				cached = cached[1:]
				for _, counter := range readCounters {
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
				destination, err = readWaiter.WaitReadPacket()
				if err != nil {
					buffer.Release()
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
				response, err := d.router.Exchange(adapter.WithContext(ctx, &metadataInQuery), &message)
				if err != nil {
					cancel(err)
					return err
				}
				timeout.Update()
				responseBuffer := buf.NewPacket()
				n, err := response.PackBuffer(responseBuffer.FreeBytes())
				if err != nil {
					cancel(err)
					responseBuffer.Release()
					return err
				}
				responseBuffer.Truncate(len(n))
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
