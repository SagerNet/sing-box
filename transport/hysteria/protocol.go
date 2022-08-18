package hysteria

import (
	"bytes"
	"encoding/binary"
	"io"
	"math/rand"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/lucas-clemente/quic-go"
)

const Version = 3

type ClientHello struct {
	SendBPS uint64
	RecvBPS uint64
	Auth    []byte
}

type ServerHello struct {
	OK      bool
	SendBPS uint64
	RecvBPS uint64
	Message string
}

func WriteClientHello(stream io.Writer, hello ClientHello) error {
	var requestLen int
	requestLen += 1 // version
	requestLen += 8 // sendBPS
	requestLen += 8 // recvBPS
	requestLen += 2 // auth len
	requestLen += len(hello.Auth)
	_request := buf.StackNewSize(requestLen)
	defer common.KeepAlive(_request)
	request := common.Dup(_request)
	defer request.Release()
	common.Must(
		request.WriteByte(Version),
		binary.Write(request, binary.BigEndian, hello.SendBPS),
		binary.Write(request, binary.BigEndian, hello.RecvBPS),
		binary.Write(request, binary.BigEndian, uint16(len(hello.Auth))),
		common.Error(request.Write(hello.Auth)),
	)
	return common.Error(stream.Write(request.Bytes()))
}

func ReadServerHello(stream io.Reader) (*ServerHello, error) {
	var responseLen int
	responseLen += 1 // ok
	responseLen += 8 // sendBPS
	responseLen += 8 // recvBPS
	responseLen += 2 // message len
	_response := buf.StackNewSize(responseLen)
	defer common.KeepAlive(_response)
	response := common.Dup(_response)
	defer response.Release()
	_, err := response.ReadFullFrom(stream, responseLen)
	if err != nil {
		return nil, err
	}
	var serverHello ServerHello
	serverHello.OK = response.Byte(0) == 1
	serverHello.SendBPS = binary.BigEndian.Uint64(response.Range(1, 9))
	serverHello.RecvBPS = binary.BigEndian.Uint64(response.Range(9, 17))
	messageLen := binary.BigEndian.Uint16(response.Range(17, 19))
	if messageLen == 0 {
		return &serverHello, nil
	}
	message := make([]byte, messageLen)
	_, err = io.ReadFull(stream, message)
	if err != nil {
		return nil, err
	}
	serverHello.Message = string(message)
	return &serverHello, nil
}

type ClientRequest struct {
	UDP  bool
	Host string
	Port uint16
}

type ServerResponse struct {
	OK           bool
	UDPSessionID uint32
	Message      string
}

func WriteClientRequest(stream io.Writer, request ClientRequest, payload []byte) error {
	var requestLen int
	requestLen += 1 // udp
	requestLen += 2 // host len
	requestLen += len(request.Host)
	requestLen += 2 // port
	requestLen += len(payload)
	_buffer := buf.StackNewSize(requestLen)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	if request.UDP {
		common.Must(buffer.WriteByte(1))
	} else {
		common.Must(buffer.WriteByte(0))
	}
	common.Must(
		binary.Write(buffer, binary.BigEndian, uint16(len(request.Host))),
		common.Error(buffer.WriteString(request.Host)),
		binary.Write(buffer, binary.BigEndian, request.Port),
		common.Error(buffer.Write(payload)),
	)
	return common.Error(stream.Write(buffer.Bytes()))
}

func ReadServerResponse(stream io.Reader) (*ServerResponse, error) {
	var responseLen int
	responseLen += 1 // ok
	responseLen += 4 // udp session id
	responseLen += 2 // message len
	_response := buf.StackNewSize(responseLen)
	defer common.KeepAlive(_response)
	response := common.Dup(_response)
	defer response.Release()
	_, err := response.ReadFullFrom(stream, responseLen)
	if err != nil {
		return nil, err
	}
	var serverResponse ServerResponse
	serverResponse.OK = response.Byte(0) == 1
	serverResponse.UDPSessionID = binary.BigEndian.Uint32(response.Range(1, 5))
	messageLen := binary.BigEndian.Uint16(response.Range(5, 7))
	if messageLen == 0 {
		return &serverResponse, nil
	}
	message := make([]byte, messageLen)
	_, err = io.ReadFull(stream, message)
	if err != nil {
		return nil, err
	}
	serverResponse.Message = string(message)
	return &serverResponse, nil
}

type UDPMessage struct {
	SessionID uint32
	Host      string
	Port      uint16
	MsgID     uint16 // doesn't matter when not fragmented, but must not be 0 when fragmented
	FragID    uint8  // doesn't matter when not fragmented, starts at 0 when fragmented
	FragCount uint8  // must be 1 when not fragmented
	Data      []byte
}

func (m UDPMessage) HeaderSize() int {
	return 4 + 2 + len(m.Host) + 2 + 2 + 1 + 1 + 2
}

func (m UDPMessage) Size() int {
	return m.HeaderSize() + len(m.Data)
}

func ParseUDPMessage(packet []byte) (message UDPMessage, err error) {
	reader := bytes.NewReader(packet)
	err = binary.Read(reader, binary.BigEndian, &message.SessionID)
	if err != nil {
		return
	}
	var hostLen uint16
	err = binary.Read(reader, binary.BigEndian, &hostLen)
	if err != nil {
		return
	}
	_, err = reader.Seek(int64(hostLen), io.SeekCurrent)
	if err != nil {
		return
	}
	message.Host = string(packet[6 : 6+hostLen])
	err = binary.Read(reader, binary.BigEndian, &message.Port)
	if err != nil {
		return
	}
	err = binary.Read(reader, binary.BigEndian, &message.MsgID)
	if err != nil {
		return
	}
	err = binary.Read(reader, binary.BigEndian, &message.FragID)
	if err != nil {
		return
	}
	err = binary.Read(reader, binary.BigEndian, &message.FragCount)
	if err != nil {
		return
	}
	var dataLen uint16
	err = binary.Read(reader, binary.BigEndian, &dataLen)
	if err != nil {
		return
	}
	if reader.Len() != int(dataLen) {
		err = E.New("invalid data length")
	}
	dataOffset := int(reader.Size()) - reader.Len()
	message.Data = packet[dataOffset:]
	return
}

func WriteUDPMessage(conn quic.Connection, message UDPMessage) error {
	var messageLen int
	messageLen += 4 // session id
	messageLen += 2 // host len
	messageLen += len(message.Host)
	messageLen += 2 // port
	messageLen += 2 // msg id
	messageLen += 1 // frag id
	messageLen += 1 // frag count
	messageLen += 2 // data len
	messageLen += len(message.Data)
	_buffer := buf.StackNewSize(messageLen)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	err := writeUDPMessage(conn, message, buffer)
	// TODO: wait for change upstream
	if /*errSize, ok := err.(quic.ErrMessageToLarge); ok*/ false {
		const errSize = 0
		// need to frag
		message.MsgID = uint16(rand.Intn(0xFFFF)) + 1 // msgID must be > 0 when fragCount > 1
		fragMsgs := FragUDPMessage(message, int(errSize))
		for _, fragMsg := range fragMsgs {
			buffer.FullReset()
			err = writeUDPMessage(conn, fragMsg, buffer)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return err
}

func writeUDPMessage(conn quic.Connection, message UDPMessage, buffer *buf.Buffer) error {
	common.Must(
		binary.Write(buffer, binary.BigEndian, message.SessionID),
		binary.Write(buffer, binary.BigEndian, uint16(len(message.Host))),
		common.Error(buffer.WriteString(message.Host)),
		binary.Write(buffer, binary.BigEndian, message.Port),
		binary.Write(buffer, binary.BigEndian, message.MsgID),
		binary.Write(buffer, binary.BigEndian, message.FragID),
		binary.Write(buffer, binary.BigEndian, message.FragCount),
		binary.Write(buffer, binary.BigEndian, uint16(len(message.Data))),
		common.Error(buffer.Write(message.Data)),
	)
	return conn.SendMessage(buffer.Bytes())
}
