package libbox

import (
	"encoding/binary"
	"net"
	"path/filepath"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

type CommandClient struct {
	sharedDirectory string
	handler         CommandClientHandler
	conn            net.Conn
	options         CommandClientOptions
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

func NewStandaloneCommandClient(sharedDirectory string) *CommandClient {
	return &CommandClient{
		sharedDirectory: sharedDirectory,
	}
}

func NewCommandClient(sharedDirectory string, handler CommandClientHandler, options *CommandClientOptions) *CommandClient {
	return &CommandClient{
		sharedDirectory: sharedDirectory,
		handler:         handler,
		options:         common.PtrValueOrDefault(options),
	}
}

func (c *CommandClient) directConnect() (net.Conn, error) {
	return net.DialUnix("unix", nil, &net.UnixAddr{
		Name: filepath.Join(c.sharedDirectory, "command.sock"),
		Net:  "unix",
	})
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
