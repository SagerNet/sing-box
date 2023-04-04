package libbox

import (
	"encoding/binary"
	"net"
	"runtime/debug"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/rw"
)

func ClientServiceStop(sharedDirectory string) error {
	conn, err := clientConnect(sharedDirectory)
	if err != nil {
		return err
	}
	defer conn.Close()
	err = binary.Write(conn, binary.BigEndian, uint8(CommandServiceStop))
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

func (s *CommandServer) handleServiceStop(conn net.Conn) error {
	rErr := s.handler.ServiceStop()
	err := binary.Write(conn, binary.BigEndian, rErr != nil)
	if err != nil {
		return err
	}
	if rErr != nil {
		return rw.WriteVString(conn, rErr.Error())
	}
	debug.FreeOSMemory()
	return nil
}
