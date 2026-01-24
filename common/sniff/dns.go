package sniff

import (
	"context"
	"encoding/binary"
	"io"
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"

	mDNS "github.com/miekg/dns"
)

func StreamDomainNameQuery(readCtx context.Context, metadata *adapter.InboundContext, reader io.Reader) error {
	var length uint16
	err := binary.Read(reader, binary.BigEndian, &length)
	if err != nil {
		return E.Cause1(ErrNeedMoreData, err)
	}
	if length < 12 {
		return os.ErrInvalid
	}
	buffer := buf.NewSize(int(length))
	defer buffer.Release()
	var n int
	n, err = buffer.ReadFullFrom(reader, buffer.FreeLen())
	packet := buffer.Bytes()
	if n > 2 && packet[2]&0x80 != 0 { // QR
		return os.ErrInvalid
	}
	if n > 5 && packet[4] == 0 && packet[5] == 0 { // QDCOUNT
		return os.ErrInvalid
	}
	for i := 6; i < 10; i++ {
		// ANCOUNT, NSCOUNT
		if n > i && packet[i] != 0 {
			return os.ErrInvalid
		}
	}
	if err != nil {
		return E.Cause1(ErrNeedMoreData, err)
	}
	return DomainNameQuery(readCtx, metadata, packet)
}

func DomainNameQuery(ctx context.Context, metadata *adapter.InboundContext, packet []byte) error {
	var msg mDNS.Msg
	err := msg.Unpack(packet)
	if err != nil || msg.Response || len(msg.Question) == 0 || len(msg.Answer) > 0 || len(msg.Ns) > 0 {
		return err
	}
	metadata.Protocol = C.ProtocolDNS
	return nil
}
