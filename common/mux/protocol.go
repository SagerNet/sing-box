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
)

var Destination = M.Socksaddr{
	Fqdn: "sp.mux.sing-box.arpa",
	Port: 444,
}

func newMuxConfig() *yamux.Config {
	config := yamux.DefaultConfig()
	config.LogOutput = io.Discard
	config.StreamCloseTimeout = C.TCPTimeout
	config.StreamOpenTimeout = C.TCPTimeout
	return config
}

const (
	version0      = 0
	flagUDP       = 1
	flagAddr      = 2
	statusSuccess = 0
	statusError   = 1
)

type Request struct {
	Network     string
	Destination M.Socksaddr
	PacketAddr  bool
}

func ReadRequest(reader io.Reader) (*Request, error) {
	version, err := rw.ReadByte(reader)
	if err != nil {
		return nil, err
	}
	if version != version0 {
		return nil, E.New("unsupported version: ", version)
	}
	var flags uint16
	err = binary.Read(reader, binary.BigEndian, &flags)
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
	return &Request{network, destination, udpAddr}, nil
}

func requestLen(request Request) int {
	var rLen int
	rLen += 1 // version
	rLen += 2 // flags
	rLen += M.SocksaddrSerializer.AddrPortLen(request.Destination)
	return rLen
}

func EncodeRequest(request Request, buffer *buf.Buffer) {
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
		buffer.WriteByte(version0),
		binary.Write(buffer, binary.BigEndian, flags),
		M.SocksaddrSerializer.WriteAddrPort(buffer, destination),
	)
}

type Response struct {
	Status  uint8
	Message string
}

func ReadResponse(reader io.Reader) (*Response, error) {
	var response Response
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
