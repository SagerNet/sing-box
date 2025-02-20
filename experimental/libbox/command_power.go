package libbox

import (
	"encoding/binary"
	"net"

	"github.com/sagernet/sing/common/varbin"
)

func (c *CommandClient) ServiceReload() error {
	conn, err := c.directConnect()
	if err != nil {
		return err
	}
	defer conn.Close()
	err = binary.Write(conn, binary.BigEndian, uint8(CommandServiceReload))
	if err != nil {
		return err
	}
	return readError(conn)
}

func (s *CommandServer) handleServiceReload(conn net.Conn) error {
	rErr := s.handler.ServiceReload()
	err := binary.Write(conn, binary.BigEndian, rErr != nil)
	if err != nil {
		return err
	}
	if rErr != nil {
		return varbin.Write(conn, binary.BigEndian, rErr.Error())
	}
	return nil
}

func (c *CommandClient) ServiceClose() error {
	conn, err := c.directConnect()
	if err != nil {
		return err
	}
	defer conn.Close()
	err = binary.Write(conn, binary.BigEndian, uint8(CommandServiceClose))
	if err != nil {
		return err
	}
	return readError(conn)
}

func (s *CommandServer) handleServiceClose(conn net.Conn) error {
	rErr := s.service.Close()
	s.handler.PostServiceClose()
	err := binary.Write(conn, binary.BigEndian, rErr != nil)
	if err != nil {
		return err
	}
	if rErr != nil {
		return varbin.Write(conn, binary.BigEndian, rErr.Error())
	}
	return nil
}
