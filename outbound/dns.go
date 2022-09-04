package outbound

import (
	"context"
	"encoding/binary"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/canceler"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/task"

	"golang.org/x/net/dns/dnsmessage"
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
	_buffer := buf.StackNewSize(int(queryLength))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	_, err = buffer.ReadFullFrom(conn, int(queryLength))
	if err != nil {
		return err
	}
	var message dnsmessage.Message
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
		_responseBuffer := buf.StackNewPacket()
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
	return nil
}

func (d *DNS) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	ctx = adapter.WithContext(ctx, &metadata)
	fastClose, cancel := context.WithCancel(ctx)
	timeout := canceler.New(fastClose, cancel, C.DNSTimeout)
	var group task.Group
	group.Append0(func(ctx context.Context) error {
		defer cancel()
		_buffer := buf.StackNewSize(dns.FixedPacketSize)
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
			timeout.Update()
			metadataInQuery := metadata
			go func() error {
				response, err := d.router.Exchange(adapter.WithContext(ctx, &metadataInQuery), &message)
				if err != nil {
					return err
				}
				timeout.Update()
				responseBuffer := buf.NewPacket()
				n, err := response.AppendPack(responseBuffer.Index(0))
				if err != nil {
					responseBuffer.Release()
					return err
				}
				responseBuffer.Truncate(len(n))
				err = conn.WritePacket(responseBuffer, destination)
				return err
			}()
		}
	})
	group.Cleanup(func() {
		conn.Close()
	})
	return group.Run(fastClose)
}
