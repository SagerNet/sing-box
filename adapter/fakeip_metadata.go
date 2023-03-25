package adapter

import (
	"bytes"
	"encoding"
	"encoding/binary"
	"io"
	"net/netip"

	"github.com/sagernet/sing/common"
)

type FakeIPMetadata struct {
	Inet4Range   netip.Prefix
	Inet6Range   netip.Prefix
	Inet4Current netip.Addr
	Inet6Current netip.Addr
}

func (m *FakeIPMetadata) MarshalBinary() (data []byte, err error) {
	var buffer bytes.Buffer
	for _, marshaler := range []encoding.BinaryMarshaler{m.Inet4Range, m.Inet6Range, m.Inet4Current, m.Inet6Current} {
		data, err = marshaler.MarshalBinary()
		if err != nil {
			return
		}
		common.Must(binary.Write(&buffer, binary.BigEndian, uint16(len(data))))
		buffer.Write(data)
	}
	data = buffer.Bytes()
	return
}

func (m *FakeIPMetadata) UnmarshalBinary(data []byte) error {
	reader := bytes.NewReader(data)
	for _, unmarshaler := range []encoding.BinaryUnmarshaler{&m.Inet4Range, &m.Inet6Range, &m.Inet4Current, &m.Inet6Current} {
		var length uint16
		common.Must(binary.Read(reader, binary.BigEndian, &length))
		element := make([]byte, length)
		_, err := io.ReadFull(reader, element)
		if err != nil {
			return err
		}
		err = unmarshaler.UnmarshalBinary(element)
		if err != nil {
			return err
		}
	}
	return nil
}
