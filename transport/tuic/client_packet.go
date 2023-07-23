package tuic

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

func (c *Client) loopMessages(conn *clientQUICConnection) {
	for {
		message, err := conn.quicConn.ReceiveMessage()
		if err != nil {
			conn.closeWithError(E.Cause(err, "receive message"))
			return
		}
		go func() {
			hErr := c.handleMessage(conn, message)
			if hErr != nil {
				conn.closeWithError(E.Cause(hErr, "handle message"))
			}
		}()
	}
}

func (c *Client) handleMessage(conn *clientQUICConnection, data []byte) error {
	if len(data) < 2 {
		return E.New("invalid message")
	}
	if data[0] != Version {
		return E.New("unknown version ", data[0])
	}
	switch data[1] {
	case CommandPacket:
		message := udpMessagePool.Get().(*udpMessage)
		err := decodeUDPMessage(message, bytes.NewReader(data[2:]))
		if err != nil {
			message.release()
			return E.Cause(err, "decode UDP message")
		}
		conn.handleUDPMessage(message)
		return nil
	case CommandHeartbeat:
		return nil
	default:
		return E.New("unknown command ", data[0])
	}
}

func (c *Client) loopUniStreams(conn *clientQUICConnection) {
	for {
		stream, err := conn.quicConn.AcceptUniStream(c.ctx)
		if err != nil {
			conn.closeWithError(E.Cause(err, "handle uni stream"))
			return
		}
		go func() {
			hErr := c.handleUniStream(conn, stream)
			if hErr != nil {
				conn.closeWithError(hErr)
			}
		}()
	}
}

func (c *Client) handleUniStream(conn *clientQUICConnection, stream quic.ReceiveStream) error {
	defer stream.CancelRead(0)
	buffer := buf.NewPacket()
	defer buffer.Release()
	_, err := buffer.ReadAtLeastFrom(stream, 2)
	if err != nil {
		return err
	}
	version, _ := buffer.ReadByte()
	if version != Version {
		return E.New("unknown version ", version)
	}
	command, _ := buffer.ReadByte()
	if command != CommandPacket {
		return E.New("unknown command ", command)
	}
	reader := io.MultiReader(bufio.NewCachedReader(stream, buffer), stream)
	message := udpMessagePool.Get().(*udpMessage)
	err = decodeUDPMessage(message, reader)
	if err != nil {
		message.release()
		return err
	}
	conn.handleUDPMessage(message)
	return nil
}

func decodeUDPMessage(message *udpMessage, reader io.Reader) error {
	err := binary.Read(reader, binary.BigEndian, &message.sessionID)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &message.packetID)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &message.fragmentTotal)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &message.fragmentID)
	if err != nil {
		return err
	}
	message.destination, err = M.SocksaddrSerializer.ReadAddrPort(reader)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &message.dataLength)
	if err != nil {
		return err
	}
	message.data = buf.NewSize(int(message.dataLength))
	_, err = message.data.ReadFullFrom(reader, message.data.FreeLen())
	if err != nil {
		return err
	}
	return nil
}
