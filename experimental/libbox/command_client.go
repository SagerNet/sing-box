package libbox

import (
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"time"

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
	ClearLogs()
	WriteLogs(messageList StringIterator)
	WriteStatus(message *StatusMessage)
	WriteGroups(message OutboundGroupIterator)
	InitializeClashMode(modeList StringIterator, currentMode string)
	UpdateClashMode(newMode string)
	WriteConnections(message *Connections)
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

func (c *CommandClient) directConnectWithRetry() (net.Conn, error) {
	var (
		conn net.Conn
		err  error
	)
	for i := 0; i < 10; i++ {
		conn, err = c.directConnect()
		if err == nil {
			return conn, nil
		}
		time.Sleep(time.Duration(100+i*50) * time.Millisecond)
	}
	return nil, err
}

func (c *CommandClient) Connect() error {
	common.Close(c.conn)
	conn, err := c.directConnectWithRetry()
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
		err = binary.Write(conn, binary.BigEndian, c.options.StatusInterval)
		if err != nil {
			return E.Cause(err, "write interval")
		}
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
	case CommandClashMode:
		var (
			modeList    []string
			currentMode string
		)
		modeList, currentMode, err = readClashModeList(conn)
		if err != nil {
			return err
		}
		if sFixAndroidStack {
			go func() {
				c.handler.Connected()
				c.handler.InitializeClashMode(newIterator(modeList), currentMode)
				if len(modeList) == 0 {
					conn.Close()
					c.handler.Disconnected(os.ErrInvalid.Error())
				}
			}()
		} else {
			c.handler.Connected()
			c.handler.InitializeClashMode(newIterator(modeList), currentMode)
			if len(modeList) == 0 {
				conn.Close()
				c.handler.Disconnected(os.ErrInvalid.Error())
			}
		}
		if len(modeList) == 0 {
			return nil
		}
		go c.handleModeConn(conn)
	case CommandConnections:
		err = binary.Write(conn, binary.BigEndian, c.options.StatusInterval)
		if err != nil {
			return E.Cause(err, "write interval")
		}
		c.handler.Connected()
		go c.handleConnectionsConn(conn)
	}
	return nil
}

func (c *CommandClient) Disconnect() error {
	return common.Close(c.conn)
}
