package sniff

import (
	"context"
	"encoding/binary"
	"io"
	"os"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/task"

	mDNS "github.com/miekg/dns"
)

func StreamDomainNameQuery(readCtx context.Context, metadata *adapter.InboundContext, reader io.Reader) error {
	var length uint16
	err := binary.Read(reader, binary.BigEndian, &length)
	if err != nil {
		return os.ErrInvalid
	}
	if length == 0 {
		return os.ErrInvalid
	}
	buffer := buf.NewSize(int(length))
	defer buffer.Release()
	readCtx, cancel := context.WithTimeout(readCtx, time.Millisecond*100)
	var readTask task.Group
	readTask.Append0(func(ctx context.Context) error {
		return common.Error(buffer.ReadFullFrom(reader, buffer.FreeLen()))
	})
	err = readTask.Run(readCtx)
	cancel()
	if err != nil {
		return err
	}
	return DomainNameQuery(readCtx, metadata, buffer.Bytes())
}

func DomainNameQuery(ctx context.Context, metadata *adapter.InboundContext, packet []byte) error {
	var msg mDNS.Msg
	err := msg.Unpack(packet)
	if err != nil {
		return err
	}
	if len(msg.Question) == 0 || msg.Question[0].Qclass != mDNS.ClassINET || !M.IsDomainName(msg.Question[0].Name) {
		return os.ErrInvalid
	}
	metadata.Protocol = C.ProtocolDNS
	return nil
}
