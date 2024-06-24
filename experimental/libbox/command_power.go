package libbox

import (
	"encoding/binary"
	"net"

	E "github.com/sagernet/sing/common/exceptions"
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
	var hasError bool
	err = binary.Read(conn, binary.BigEndian, &hasError)
	if err != nil {
		return err
	}
	if hasError {
		errorMessage, err := varbin.ReadValue[string](conn, binary.BigEndian)
		if err != nil {
			return err
		}
		return E.New(errorMessage)
	}
	return nil
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
	var hasError bool
	err = binary.Read(conn, binary.BigEndian, &hasError)
	if err != nil {
		return nil
	}
	if hasError {
		errorMessage, err := varbin.ReadValue[string](conn, binary.BigEndian)
		if err != nil {
			return nil
		}
		return E.New(errorMessage)
	}
	return nil
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
