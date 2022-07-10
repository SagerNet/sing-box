package inbound

import (
	"context"
	"encoding/binary"
	"io"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/net/dns/dnsmessage"
)

func NewDNSConnection(ctx context.Context, router adapter.Router, logger log.Logger, conn net.Conn, metadata adapter.InboundContext) error {
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
			logger.WithContext(ctx).Debug("inbound dns query ", formatDNSQuestion(question), " from ", metadata.Source)
		}
		response, err := router.Exchange(adapter.WithContext(ctx, &metadata), &message)
		if err != nil {
			return err
		}
		buffer.FullReset()
		responseBuffer, err := response.AppendPack(buffer.Index(0))
		if err != nil {
			return err
		}
		err = binary.Write(conn, binary.BigEndian, uint16(len(responseBuffer)))
		if err != nil {
			return err
		}
		_, err = conn.Write(responseBuffer)
		if err != nil {
			return err
		}
	}
}

func NewDNSPacketConnection(ctx context.Context, router adapter.Router, logger log.Logger, conn N.PacketConn, metadata adapter.InboundContext) error {
	for {
		buffer := buf.StackNewSize(1024)
		destination, err := conn.ReadPacket(buffer)
		if err != nil {
			buffer.Release()
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
			logger.WithContext(ctx).Debug("inbound dns query ", formatDNSQuestion(question), " from ", metadata.Source)
		}
		go func() error {
			defer buffer.Release()
			response, err := router.Exchange(adapter.WithContext(ctx, &metadata), &message)
			if err != nil {
				return err
			}
			buffer.FullReset()
			responseBuffer, err := response.AppendPack(buffer.Index(0))
			if err != nil {
				return err
			}
			buffer.Truncate(len(responseBuffer))
			err = conn.WritePacket(buffer, destination)
			return err
		}()
	}
}

func formatDNSQuestion(question dnsmessage.Question) string {
	domain := question.Name.String()
	domain = domain[:len(domain)-1]
	return string(question.Name.Data[:question.Name.Length-1]) + " " + question.Type.String()[4:] + " " + question.Class.String()[5:]
}
