package hysteria

import (
	"bytes"
	"encoding/binary"
	"io"
	"math/rand"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/lucas-clemente/quic-go"
)

const (
	MbpsToBps                      = 125000
	MinSpeedBPS                    = 16384
	DefaultStreamReceiveWindow     = 15728640 // 15 MB/s
	DefaultConnectionReceiveWindow = 67108864 // 64 MB/s
	DefaultMaxIncomingStreams      = 1024
	DefaultALPN                    = "hysteria"
	KeepAlivePeriod                = 10 * time.Second
)

const Version = 3

type ClientHello struct {
	SendBPS uint64
	RecvBPS uint64
	Auth    []byte
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

func ReadClientHello(reader io.Reader) (*ClientHello, error) {
	var version uint8
	err := binary.Read(reader, binary.BigEndian, &version)
	if err != nil {
		return nil, err
	}
	if version != Version {
		return nil, E.New("unsupported client version: ", version)
	}
	var clientHello ClientHello
	err = binary.Read(reader, binary.BigEndian, &clientHello.SendBPS)
	if err != nil {
		return nil, err
	}
	err = binary.Read(reader, binary.BigEndian, &clientHello.RecvBPS)
	if err != nil {
		return nil, err
	}
	var authLen uint16
	err = binary.Read(reader, binary.BigEndian, &authLen)
	if err != nil {
		return nil, err
	}
	clientHello.Auth = make([]byte, authLen)
	_, err = io.ReadFull(reader, clientHello.Auth)
	if err != nil {
		return nil, err
	}
	return &clientHello, nil
}

type ServerHello struct {
	OK      bool
	SendBPS uint64
	RecvBPS uint64
	Message string
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

func WriteServerHello(stream io.Writer, hello ServerHello) error {
	var responseLen int
	responseLen += 1 // ok
	responseLen += 8 // sendBPS
	responseLen += 8 // recvBPS
	responseLen += 2 // message len
	responseLen += len(hello.Message)
	_response := buf.StackNewSize(responseLen)
	defer common.KeepAlive(_response)
	response := common.Dup(_response)
	defer response.Release()
	if hello.OK {
		common.Must(response.WriteByte(1))
	} else {
		common.Must(response.WriteByte(0))
	}
	common.Must(
		binary.Write(response, binary.BigEndian, hello.SendBPS),
		binary.Write(response, binary.BigEndian, hello.RecvBPS),
		binary.Write(response, binary.BigEndian, uint16(len(hello.Message))),
		common.Error(response.WriteString(hello.Message)),
	)
	return common.Error(stream.Write(response.Bytes()))
}

type ClientRequest struct {
	UDP  bool
	Host string
	Port uint16
}

func ReadClientRequest(stream io.Reader) (*ClientRequest, error) {
	var clientRequest ClientRequest
	err := binary.Read(stream, binary.BigEndian, &clientRequest.UDP)
	if err != nil {
		return nil, err
	}
	var hostLen uint16
	err = binary.Read(stream, binary.BigEndian, &hostLen)
	if err != nil {
		return nil, err
	}
	host := make([]byte, hostLen)
	_, err = io.ReadFull(stream, host)
	if err != nil {
		return nil, err
	}
	clientRequest.Host = string(host)
	err = binary.Read(stream, binary.BigEndian, &clientRequest.Port)
	if err != nil {
		return nil, err
	}
	return &clientRequest, nil
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

type ServerResponse struct {
	OK           bool
	UDPSessionID uint32
	Message      string
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

func WriteServerResponse(stream io.Writer, response ServerResponse, payload []byte) error {
	var responseLen int
	responseLen += 1 // ok
	responseLen += 4 // udp session id
	responseLen += 2 // message len
	responseLen += len(response.Message)
	responseLen += len(payload)
	_buffer := buf.StackNewSize(responseLen)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	if response.OK {
		common.Must(buffer.WriteByte(1))
	} else {
		common.Must(buffer.WriteByte(0))
	}
	common.Must(
		binary.Write(buffer, binary.BigEndian, response.UDPSessionID),
		binary.Write(buffer, binary.BigEndian, uint16(len(response.Message))),
		common.Error(buffer.WriteString(response.Message)),
		common.Error(buffer.Write(payload)),
	)
	return common.Error(stream.Write(buffer.Bytes()))
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
