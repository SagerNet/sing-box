//go:build darwin

package libbox

import (
	"net"
	"path/filepath"

	"github.com/sagernet/sing/common"
)

type LogClient struct {
	sockPath string
	handler  LogClientHandler
	conn     net.Conn
}

type LogClientHandler interface {
	Connected()
	Disconnected()
	WriteLog(message string)
}

func NewLogClient(sharedDirectory string, handler LogClientHandler) *LogClient {
	return &LogClient{
		sockPath: filepath.Join(sharedDirectory, "log.sock"),
		handler:  handler,
	}
}

func (c *LogClient) Connect() error {
	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{
		Name: c.sockPath,
		Net:  "unix",
	})
	if err != nil {
		return err
	}
	c.conn = conn
	go c.loopConnection(&messageConn{conn})
	return nil
}

func (c *LogClient) Disconnect() error {
	return common.Close(c.conn)
}

func (c *LogClient) loopConnection(conn *messageConn) {
	c.handler.Connected()
	defer c.handler.Disconnected()
	for {
		message, err := conn.Read()
		if err != nil {
			c.handler.WriteLog("(log client error) " + err.Error())
			return
		}
		c.handler.WriteLog(string(message))
	}
}
