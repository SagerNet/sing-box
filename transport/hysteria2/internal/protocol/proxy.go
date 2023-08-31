package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/sagernet/quic-go/quicvarint"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/rw"
)

const (
	FrameTypeTCPRequest = 0x401

	// Max length values are for preventing DoS attacks

	MaxAddressLength = 2048
	MaxMessageLength = 2048
	MaxPaddingLength = 4096

	MaxUDPSize = 4096

	maxVarInt1 = 63
	maxVarInt2 = 16383
	maxVarInt4 = 1073741823
	maxVarInt8 = 4611686018427387903
)

// TCPRequest format:
// 0x401 (QUIC varint)
// Address length (QUIC varint)
// Address (bytes)
// Padding length (QUIC varint)
// Padding (bytes)

func ReadTCPRequest(r io.Reader) (string, error) {
	bReader := quicvarint.NewReader(r)
	addrLen, err := quicvarint.Read(bReader)
	if err != nil {
		return "", err
	}
	if addrLen == 0 || addrLen > MaxAddressLength {
		return "", E.New("invalid address length")
	}
	addrBuf := make([]byte, addrLen)
	_, err = io.ReadFull(r, addrBuf)
	if err != nil {
		return "", err
	}
	paddingLen, err := quicvarint.Read(bReader)
	if err != nil {
		return "", err
	}
	if paddingLen > MaxPaddingLength {
		return "", E.New("invalid padding length")
	}
	if paddingLen > 0 {
		_, err = io.CopyN(io.Discard, r, int64(paddingLen))
		if err != nil {
			return "", err
		}
	}
	return string(addrBuf), nil
}

func WriteTCPRequest(addr string, payload []byte) *buf.Buffer {
	padding := tcpRequestPadding.String()
	paddingLen := len(padding)
	addrLen := len(addr)
	sz := int(quicvarint.Len(FrameTypeTCPRequest)) +
		int(quicvarint.Len(uint64(addrLen))) + addrLen +
		int(quicvarint.Len(uint64(paddingLen))) + paddingLen
	buffer := buf.NewSize(sz + len(payload))
	bufferContent := buffer.Extend(sz)
	i := varintPut(bufferContent, FrameTypeTCPRequest)
	i += varintPut(bufferContent[i:], uint64(addrLen))
	i += copy(bufferContent[i:], addr)
	i += varintPut(bufferContent[i:], uint64(paddingLen))
	copy(bufferContent[i:], padding)
	buffer.Write(payload)
	return buffer
}

// TCPResponse format:
// Status (byte, 0=ok, 1=error)
// Message length (QUIC varint)
// Message (bytes)
// Padding length (QUIC varint)
// Padding (bytes)

func ReadTCPResponse(r io.Reader) (bool, string, error) {
	var status [1]byte
	if _, err := io.ReadFull(r, status[:]); err != nil {
		return false, "", err
	}
	bReader := quicvarint.NewReader(r)
	msg, err := ReadVString(bReader)
	if err != nil {
		return false, "", err
	}
	paddingLen, err := quicvarint.Read(bReader)
	if err != nil {
		return false, "", err
	}
	if paddingLen > MaxPaddingLength {
		return false, "", E.New("invalid padding length")
	}
	if paddingLen > 0 {
		_, err = io.CopyN(io.Discard, r, int64(paddingLen))
		if err != nil {
			return false, "", err
		}
	}
	return status[0] == 0, msg, nil
}

func WriteTCPResponse(ok bool, msg string, payload []byte) *buf.Buffer {
	padding := tcpResponsePadding.String()
	paddingLen := len(padding)
	msgLen := len(msg)
	sz := 1 + int(quicvarint.Len(uint64(msgLen))) + msgLen +
		int(quicvarint.Len(uint64(paddingLen))) + paddingLen
	buffer := buf.NewSize(sz + len(payload))
	if ok {
		buffer.WriteByte(0)
	} else {
		buffer.WriteByte(1)
	}
	WriteVString(buffer, msg)
	WriteUVariant(buffer, uint64(paddingLen))
	buffer.Extend(paddingLen)
	buffer.Write(payload)
	return buffer
}

// UDPMessage format:
// Session ID (uint32 BE)
// Packet ID (uint16 BE)
// Fragment ID (uint8)
// Fragment count (uint8)
// Address length (QUIC varint)
// Address (bytes)
// Data...

type UDPMessage struct {
	SessionID uint32 // 4
	PacketID  uint16 // 2
	FragID    uint8  // 1
	FragCount uint8  // 1
	Addr      string // varint + bytes
	Data      []byte
}

func (m *UDPMessage) HeaderSize() int {
	lAddr := len(m.Addr)
	return 4 + 2 + 1 + 1 + int(quicvarint.Len(uint64(lAddr))) + lAddr
}

func (m *UDPMessage) Size() int {
	return m.HeaderSize() + len(m.Data)
}

func (m *UDPMessage) Serialize(buf []byte) int {
	// Make sure the buffer is big enough
	if len(buf) < m.Size() {
		return -1
	}
	binary.BigEndian.PutUint32(buf, m.SessionID)
	binary.BigEndian.PutUint16(buf[4:], m.PacketID)
	buf[6] = m.FragID
	buf[7] = m.FragCount
	i := varintPut(buf[8:], uint64(len(m.Addr)))
	i += copy(buf[8+i:], m.Addr)
	i += copy(buf[8+i:], m.Data)
	return 8 + i
}

func ParseUDPMessage(msg []byte) (*UDPMessage, error) {
	m := &UDPMessage{}
	buf := bytes.NewBuffer(msg)
	if err := binary.Read(buf, binary.BigEndian, &m.SessionID); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &m.PacketID); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &m.FragID); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &m.FragCount); err != nil {
		return nil, err
	}
	lAddr, err := quicvarint.Read(buf)
	if err != nil {
		return nil, err
	}
	if lAddr == 0 || lAddr > MaxMessageLength {
		return nil, E.New("invalid address length")
	}
	bs := buf.Bytes()
	m.Addr = string(bs[:lAddr])
	m.Data = bs[lAddr:]
	return m, nil
}

func ReadVString(reader io.Reader) (string, error) {
	length, err := quicvarint.Read(quicvarint.NewReader(reader))
	if err != nil {
		return "", err
	}
	value, err := rw.ReadBytes(reader, int(length))
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func WriteVString(writer io.Writer, value string) error {
	err := WriteUVariant(writer, uint64(len(value)))
	if err != nil {
		return err
	}
	return rw.WriteString(writer, value)
}

func WriteUVariant(writer io.Writer, value uint64) error {
	var b [8]byte
	return common.Error(writer.Write(b[:varintPut(b[:], value)]))
}

// varintPut is like quicvarint.Append, but instead of appending to a slice,
// it writes to a fixed-size buffer. Returns the number of bytes written.
func varintPut(b []byte, i uint64) int {
	if i <= maxVarInt1 {
		b[0] = uint8(i)
		return 1
	}
	if i <= maxVarInt2 {
		b[0] = uint8(i>>8) | 0x40
		b[1] = uint8(i)
		return 2
	}
	if i <= maxVarInt4 {
		b[0] = uint8(i>>24) | 0x80
		b[1] = uint8(i >> 16)
		b[2] = uint8(i >> 8)
		b[3] = uint8(i)
		return 4
	}
	if i <= maxVarInt8 {
		b[0] = uint8(i>>56) | 0xc0
		b[1] = uint8(i >> 48)
		b[2] = uint8(i >> 40)
		b[3] = uint8(i >> 32)
		b[4] = uint8(i >> 24)
		b[5] = uint8(i >> 16)
		b[6] = uint8(i >> 8)
		b[7] = uint8(i)
		return 8
	}
	panic(fmt.Sprintf("%#x doesn't fit into 62 bits", i))
}
