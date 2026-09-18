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

const dnsHeaderSize = 12

func StreamDomainNameQuery(readCtx context.Context, metadata *adapter.InboundContext, reader io.Reader) error {
	var length uint16
	err := binary.Read(reader, binary.BigEndian, &length)
	if err != nil {
		return E.Cause1(ErrNeedMoreData, err)
	}
	if length < dnsHeaderSize {
		return os.ErrInvalid
	}
	buffer := buf.NewSize(int(length))
	defer buffer.Release()
	_, err = buffer.ReadFullFrom(reader, buffer.FreeLen())
	if err != nil {
		headerErr := verifyDNSQueryHeader(buffer.Bytes())
		if headerErr != nil {
			return headerErr
		}
		return E.Cause1(ErrNeedMoreData, err)
	}
	return DomainNameQuery(readCtx, metadata, buffer.Bytes())
}

func DomainNameQuery(ctx context.Context, metadata *adapter.InboundContext, packet []byte) error {
	if len(packet) < dnsHeaderSize {
		return os.ErrInvalid
	}
	err := verifyDNSQueryHeader(packet)
	if err != nil {
		return err
	}
	offset := dnsHeaderSize
	questionCount := int(binary.BigEndian.Uint16(packet[4:6]))
	for i := 0; i < questionCount; i++ {
		_, offset, err = mDNS.UnpackDomainName(packet, offset)
		if err != nil {
			return err
		}
		if offset+4 > len(packet) {
			return os.ErrInvalid
		}
		offset += 4
	}
	additionalCount := int(binary.BigEndian.Uint16(packet[10:12]))
	for i := 0; i < additionalCount; i++ {
		_, offset, err = mDNS.UnpackRR(packet, offset)
		if err != nil {
			return err
		}
	}
	if offset != len(packet) {
		return os.ErrInvalid
	}
	metadata.Protocol = C.ProtocolDNS
	return nil
}

func verifyDNSQueryHeader(header []byte) error {
	if len(header) > 2 {
		isResponse := header[2]&0x80 != 0
		opcode := int(header[2]>>3) & 0x0F
		authoritative := header[2]&0x04 != 0
		truncated := header[2]&0x02 != 0
		if isResponse || opcode != mDNS.OpcodeQuery || authoritative || truncated {
			return os.ErrInvalid
		}
	}
	if len(header) > 3 {
		recursionAvailable := header[3]&0x80 != 0
		reserved := header[3]&0x40 != 0
		responseCode := int(header[3] & 0x0F)
		if recursionAvailable || reserved || responseCode != mDNS.RcodeSuccess {
			return os.ErrInvalid
		}
	}
	if len(header) > 5 && binary.BigEndian.Uint16(header[4:6]) == 0 {
		return os.ErrInvalid
	}
	if len(header) > 7 && binary.BigEndian.Uint16(header[6:8]) != 0 {
		return os.ErrInvalid
	}
	if len(header) > 9 && binary.BigEndian.Uint16(header[8:10]) != 0 {
		return os.ErrInvalid
	}
	return nil
}
