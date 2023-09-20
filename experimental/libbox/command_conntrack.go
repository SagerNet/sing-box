package libbox

import (
	"encoding/binary"
	"net"
	runtimeDebug "runtime/debug"
	"time"

	"github.com/sagernet/sing-box/common/conntrack"
)

func (c *CommandClient) CloseConnections() error {
	conn, err := c.directConnect()
	if err != nil {
		return err
	}
	defer conn.Close()
	return binary.Write(conn, binary.BigEndian, uint8(CommandCloseConnections))
}

func (s *CommandServer) handleCloseConnections(conn net.Conn) error {
	conntrack.Close()
	go func() {
		time.Sleep(time.Second)
		runtimeDebug.FreeOSMemory()
	}()
	return nil
}
