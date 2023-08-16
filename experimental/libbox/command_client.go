package libbox

import (
	"encoding/binary"
	"net"
	"path/filepath"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

type CommandClient struct {
	handler CommandClientHandler
	conn    net.Conn
	options CommandClientOptions
}

type CommandClientOptions struct {
	Command        int32
	StatusInterval int64
}

type CommandClientHandler interface {
	Connected()
	Disconnected(message string)
	WriteLog(message string)
	WriteStatus(message *StatusMessage)
	WriteGroups(message OutboundGroupIterator)
}

func NewStandaloneCommandClient() *CommandClient {
	return new(CommandClient)
}

func NewCommandClient(handler CommandClientHandler, options *CommandClientOptions) *CommandClient {
	return &CommandClient{
		handler: handler,
		options: common.PtrValueOrDefault(options),
	}
}

func (c *CommandClient) directConnect() (net.Conn, error) {
	if !sTVOS {
		return net.DialUnix("unix", nil, &net.UnixAddr{
			Name: filepath.Join(sBasePath, "command.sock"),
			Net:  "unix",
		})
	} else {
		return net.Dial("tcp", "127.0.0.1:8964")
	}
}

func (c *CommandClient) Connect() error {
	common.Close(c.conn)
	conn, err := c.directConnect()
	if err != nil {
		return err
	}
	c.conn = conn
	err = binary.Write(conn, binary.BigEndian, uint8(c.options.Command))
	if err != nil {
		return err
	}
	switch c.options.Command {
	case CommandLog:
		c.handler.Connected()
		go c.handleLogConn(conn)
	case CommandStatus:
		err = binary.Write(conn, binary.BigEndian, c.options.StatusInterval)
		if err != nil {
			return E.Cause(err, "write interval")
		}
		c.handler.Connected()
		go c.handleStatusConn(conn)
	case CommandGroup:
		err = binary.Write(conn, binary.BigEndian, c.options.StatusInterval)
		if err != nil {
			return E.Cause(err, "write interval")
		}
		c.handler.Connected()
		go c.handleGroupConn(conn)
	}
	return nil
}

func (c *CommandClient) Disconnect() error {
	return common.Close(c.conn)
}
