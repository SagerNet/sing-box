package libbox

import (
	"encoding/binary"
	"net"

	"github.com/sagernet/sing-box/outbound"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/rw"
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
	err = rw.WriteVString(conn, groupTag)
	if err != nil {
		return err
	}
	err = rw.WriteVString(conn, outboundTag)
	if err != nil {
		return err
	}
	return readError(conn)
}

func (s *CommandServer) handleSelectOutbound(conn net.Conn) error {
	defer conn.Close()
	groupTag, err := rw.ReadVString(conn)
	if err != nil {
		return err
	}
	outboundTag, err := rw.ReadVString(conn)
	if err != nil {
		return err
	}
	service := s.service
	if service == nil {
		return writeError(conn, E.New("service not ready"))
	}
	outboundGroup, isLoaded := service.instance.Router().Outbound(groupTag)
	if !isLoaded {
		return writeError(conn, E.New("selector not found: ", groupTag))
	}
	selector, isSelector := outboundGroup.(*outbound.Selector)
	if !isSelector {
		return writeError(conn, E.New("outbound is not a selector: ", groupTag))
	}
	if !selector.SelectOutbound(outboundTag) {
		return writeError(conn, E.New("outbound not found in selector: ", outboundTag))
	}
	return writeError(conn, nil)
}
