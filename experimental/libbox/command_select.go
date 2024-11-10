package libbox

import (
	"encoding/binary"
	"net"

	"github.com/sagernet/sing-box/protocol/group"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/varbin"
)

func (c *CommandClient) SelectOutbound(groupTag string, outboundTag string) error {
	conn, err := c.directConnect()
	if err != nil {
		return err
	}
	defer conn.Close()
	err = binary.Write(conn, binary.BigEndian, uint8(CommandSelectOutbound))
	if err != nil {
		return err
	}
	err = varbin.Write(conn, binary.BigEndian, groupTag)
	if err != nil {
		return err
	}
	err = varbin.Write(conn, binary.BigEndian, outboundTag)
	if err != nil {
		return err
	}
	return readError(conn)
}

func (s *CommandServer) handleSelectOutbound(conn net.Conn) error {
	groupTag, err := varbin.ReadValue[string](conn, binary.BigEndian)
	if err != nil {
		return err
	}
	outboundTag, err := varbin.ReadValue[string](conn, binary.BigEndian)
	if err != nil {
		return err
	}
	service := s.service
	if service == nil {
		return writeError(conn, E.New("service not ready"))
	}
	outboundGroup, isLoaded := service.instance.Outbound().Outbound(groupTag)
	if !isLoaded {
		return writeError(conn, E.New("selector not found: ", groupTag))
	}
	selector, isSelector := outboundGroup.(*group.Selector)
	if !isSelector {
		return writeError(conn, E.New("outbound is not a selector: ", groupTag))
	}
	if !selector.SelectOutbound(outboundTag) {
		return writeError(conn, E.New("outbound not found in selector: ", outboundTag))
	}
	return writeError(conn, nil)
}
