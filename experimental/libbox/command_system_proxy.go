package libbox

import (
	"encoding/binary"
	"net"
)

type SystemProxyStatus struct {
	Available bool
	Enabled   bool
}

func (c *CommandClient) GetSystemProxyStatus() (*SystemProxyStatus, error) {
	conn, err := c.directConnectWithRetry()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	err = binary.Write(conn, binary.BigEndian, uint8(CommandGetSystemProxyStatus))
	if err != nil {
		return nil, err
	}
	var status SystemProxyStatus
	err = binary.Read(conn, binary.BigEndian, &status.Available)
	if err != nil {
		return nil, err
	}
	if status.Available {
		err = binary.Read(conn, binary.BigEndian, &status.Enabled)
		if err != nil {
			return nil, err
		}
	}
	return &status, nil
}

func (s *CommandServer) handleGetSystemProxyStatus(conn net.Conn) error {
	status := s.handler.GetSystemProxyStatus()
	err := binary.Write(conn, binary.BigEndian, status.Available)
	if err != nil {
		return err
	}
	if status.Available {
		err = binary.Write(conn, binary.BigEndian, status.Enabled)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *CommandClient) SetSystemProxyEnabled(isEnabled bool) error {
	conn, err := c.directConnect()
	if err != nil {
		return err
	}
	defer conn.Close()
	err = binary.Write(conn, binary.BigEndian, uint8(CommandSetSystemProxyEnabled))
	if err != nil {
		return err
	}
	err = binary.Write(conn, binary.BigEndian, isEnabled)
	if err != nil {
		return err
	}
	return readError(conn)
}

func (s *CommandServer) handleSetSystemProxyEnabled(conn net.Conn) error {
	var isEnabled bool
	err := binary.Read(conn, binary.BigEndian, &isEnabled)
	if err != nil {
		return err
	}
	err = s.handler.SetSystemProxyEnabled(isEnabled)
	if err != nil {
		return writeError(conn, err)
	}
	return writeError(conn, nil)
}
