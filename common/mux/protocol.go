package mux

import (
	"encoding/binary"
	"io"
	"net"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"

	"github.com/hashicorp/yamux"
	"github.com/xtaci/smux"
)

var Destination = M.Socksaddr{
	Fqdn: "sp.mux.sing-box.arpa",
	Port: 444,
}

const (
	ProtocolYAMux Protocol = 0
	ProtocolSMux  Protocol = 1
)

type Protocol byte

func ParseProtocol(name string) (Protocol, error) {
	switch name {
	case "", "yamux":
		return ProtocolYAMux, nil
	case "smux":
		return ProtocolSMux, nil
	default:
		return ProtocolYAMux, E.New("unknown multiplex protocol: ", name)
	}
}

func (p Protocol) newServer(conn net.Conn) (abstractSession, error) {
	switch p {
	case ProtocolYAMux:
		return yamux.Server(conn, yaMuxConfig())
	case ProtocolSMux:
		session, err := smux.Server(conn, nil)
		if err != nil {
			return nil, err
		}
		return &smuxSession{session}, nil
	default:
		panic("unknown protocol")
	}
}

func (p Protocol) newClient(conn net.Conn) (abstractSession, error) {
	switch p {
	case ProtocolYAMux:
		return yamux.Client(conn, yaMuxConfig())
	case ProtocolSMux:
		session, err := smux.Client(conn, nil)
		if err != nil {
			return nil, err
		}
		return &smuxSession{session}, nil
	default:
		panic("unknown protocol")
	}
}

func yaMuxConfig() *yamux.Config {
	config := yamux.DefaultConfig()
	config.LogOutput = io.Discard
	config.StreamCloseTimeout = C.TCPTimeout
	config.StreamOpenTimeout = C.TCPTimeout
	return config
}

func (p Protocol) String() string {
	switch p {
	case ProtocolYAMux:
		return "yamux"
	case ProtocolSMux:
		return "smux"
	default:
		return "unknown"
	}
}

const (
	version0 = 0
)

type Request struct {
	Protocol Protocol
}

func ReadRequest(reader io.Reader) (*Request, error) {
	version, err := rw.ReadByte(reader)
	if err != nil {
		return nil, err
	}
	if version != version0 {
		return nil, E.New("unsupported version: ", version)
	}
	protocol, err := rw.ReadByte(reader)
	if err != nil {
		return nil, err
	}
	if protocol > byte(ProtocolSMux) {
		return nil, E.New("unsupported protocol: ", protocol)
	}
	return &Request{Protocol: Protocol(protocol)}, nil
}

func EncodeRequest(buffer *buf.Buffer, request Request) {
	buffer.WriteByte(version0)
	buffer.WriteByte(byte(request.Protocol))
}

const (
	flagUDP       = 1
	flagAddr      = 2
	statusSuccess = 0
	statusError   = 1
)

type StreamRequest struct {
	Network     string
	Destination M.Socksaddr
	PacketAddr  bool
}

func ReadStreamRequest(reader io.Reader) (*StreamRequest, error) {
	var flags uint16
	err := binary.Read(reader, binary.BigEndian, &flags)
	if err != nil {
		return nil, err
	}
	destination, err := M.SocksaddrSerializer.ReadAddrPort(reader)
	if err != nil {
		return nil, err
	}
	var network string
	var udpAddr bool
	if flags&flagUDP == 0 {
		network = N.NetworkTCP
	} else {
		network = N.NetworkUDP
		udpAddr = flags&flagAddr != 0
	}
	return &StreamRequest{network, destination, udpAddr}, nil
}

func requestLen(request StreamRequest) int {
	var rLen int
	rLen += 1 // version
	rLen += 2 // flags
	rLen += M.SocksaddrSerializer.AddrPortLen(request.Destination)
	return rLen
}

func EncodeStreamRequest(request StreamRequest, buffer *buf.Buffer) {
	destination := request.Destination
	var flags uint16
	if request.Network == N.NetworkUDP {
		flags |= flagUDP
	}
	if request.PacketAddr {
		flags |= flagAddr
		if !destination.IsValid() {
			destination = Destination
		}
	}
	common.Must(
		binary.Write(buffer, binary.BigEndian, flags),
		M.SocksaddrSerializer.WriteAddrPort(buffer, destination),
	)
}

type StreamResponse struct {
	Status  uint8
	Message string
}

func ReadStreamResponse(reader io.Reader) (*StreamResponse, error) {
	var response StreamResponse
	status, err := rw.ReadByte(reader)
	if err != nil {
		return nil, err
	}
	response.Status = status
	if status == statusError {
		response.Message, err = rw.ReadVString(reader)
		if err != nil {
			return nil, err
		}
	}
	return &response, nil
}

type wrapStream struct {
	net.Conn
}

func (w *wrapStream) Read(p []byte) (n int, err error) {
	n, err = w.Conn.Read(p)
	err = wrapError(err)
	return
}

func (w *wrapStream) Write(p []byte) (n int, err error) {
	n, err = w.Conn.Write(p)
	err = wrapError(err)
	return
}

func wrapError(err error) error {
	switch err {
	case yamux.ErrStreamClosed:
		return io.EOF
	default:
		return err
	}
}
