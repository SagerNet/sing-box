package wsc

import (
	"encoding"
	"encoding/binary"
	"errors"
	"net/netip"
)

const packetConnPayloadHeaderLen = 18

var _ encoding.BinaryMarshaler = &packetConnPayload{}
var _ encoding.BinaryUnmarshaler = &packetConnPayload{}

type packetConnPayload struct {
	addrPort netip.AddrPort
	payload  []byte
}

func (payload *packetConnPayload) UnmarshalBinary(data []byte) error {
	if err := payload.UnmarshalBinaryUnsafe(data); err != nil {
		return err
	}

	payload.payload = append(make([]byte, 0, len(payload.payload)), payload.payload...)

	return nil
}

func (payload *packetConnPayload) MarshalBinary() (data []byte, err error) {
	if !payload.addrPort.IsValid() {
		return nil, errors.New("addr port is not valid")
	}
	data = make([]byte, len(payload.payload)+packetConnPayloadHeaderLen)
	return data, payload.MarshalBinaryUnsafe(data)
}

func (payload *packetConnPayload) UnmarshalBinaryUnsafe(data []byte) error {
	const hLen = packetConnPayloadHeaderLen

	if len(data) < hLen {
		return errors.New("invalid payload")
	}

	addr, ok := netip.AddrFromSlice(data[:hLen-2])
	if !ok {
		return errors.New("couldn't parse addr port")
	}
	port := binary.LittleEndian.Uint16(data[hLen-2 : hLen])
	payload.addrPort = netip.AddrPortFrom(addr, port)

	payload.payload = data[hLen:]

	return nil
}

func (payload *packetConnPayload) MarshalBinaryUnsafe(data []byte) error {
	const hLen = packetConnPayloadHeaderLen

	if !payload.addrPort.IsValid() {
		return errors.New("addr port is not valid")
	}

	if len(data) < hLen+len(payload.payload) {
		return errors.New("invalid data length to write")
	}

	addr := payload.addrPort.Addr().As16()
	copy(data[:hLen-2], addr[:])

	binary.LittleEndian.PutUint16(data[hLen-2:hLen], payload.addrPort.Port())

	copy(data[hLen:], payload.payload)

	return nil
}
