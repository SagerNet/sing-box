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
	if length == 0 {
		return os.ErrInvalid
	}
	buffer := buf.NewSize(int(length))
	defer buffer.Release()
	_, err = buffer.ReadFullFrom(reader, buffer.FreeLen())
	if err != nil {
		return E.Cause1(ErrNeedMoreData, err)
	}
	return DomainNameQuery(readCtx, metadata, buffer.Bytes())
}

func DomainNameQuery(ctx context.Context, metadata *adapter.InboundContext, packet []byte) error {
	var msg mDNS.Msg
	err := msg.Unpack(packet)
	if err != nil {
		return err
	}
	metadata.Protocol = C.ProtocolDNS
	return nil
}
