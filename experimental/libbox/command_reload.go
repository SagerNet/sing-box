package libbox

import (
	"encoding/binary"
	"net"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/rw"
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
		errorMessage, err := rw.ReadVString(conn)
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
		return rw.WriteVString(conn, rErr.Error())
	}
	return nil
}
