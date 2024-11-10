package libbox

import (
	"bufio"
	"net"

	"github.com/sagernet/sing-box/experimental/clashapi"
	"github.com/sagernet/sing/common/binary"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/varbin"

	"github.com/gofrs/uuid/v5"
)

func (c *CommandClient) CloseConnection(connId string) error {
	conn, err := c.directConnect()
	if err != nil {
		return err
	}
	defer conn.Close()
	err = binary.Write(conn, binary.BigEndian, uint8(CommandCloseConnection))
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(conn)
	err = varbin.Write(writer, binary.BigEndian, connId)
	if err != nil {
		return err
	}
	err = writer.Flush()
	if err != nil {
		return err
	}
	return readError(conn)
}

func (s *CommandServer) handleCloseConnection(conn net.Conn) error {
	reader := bufio.NewReader(conn)
	var connId string
	err := varbin.Read(reader, binary.BigEndian, &connId)
	if err != nil {
		return E.Cause(err, "read connection id")
	}
	service := s.service
	if service == nil {
		return writeError(conn, E.New("service not ready"))
	}
	targetConn := service.clashServer.(*clashapi.Server).TrafficManager().Connection(uuid.FromStringOrNil(connId))
	if targetConn == nil {
		return writeError(conn, E.New("connection already closed"))
	}
	targetConn.Close()
	return writeError(conn, nil)
}
