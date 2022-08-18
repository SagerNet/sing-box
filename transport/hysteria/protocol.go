package hysteria

import (
	"encoding/binary"
	"io"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
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
