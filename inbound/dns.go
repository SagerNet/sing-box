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
