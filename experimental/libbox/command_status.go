//go:build darwin

package libbox

import (
	"encoding/binary"
	"net"
	"runtime"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
)

type StatusMessage struct {
	Memory     int64
	Goroutines int32
}

func readStatus() StatusMessage {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	var message StatusMessage
	message.Memory = int64(memStats.Sys - memStats.HeapReleased)
	message.Goroutines = int32(runtime.NumGoroutine())
	return message
}

func (s *CommandServer) handleStatusConn(conn net.Conn) error {
	var interval int64
	err := binary.Read(conn, binary.BigEndian, &interval)
	if err != nil {
		return E.Cause(err, "read interval")
	}
	ticker := time.NewTicker(time.Duration(interval))
	defer ticker.Stop()
	ctx := connKeepAlive(conn)
	for {
		err = binary.Write(conn, binary.BigEndian, readStatus())
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (c *CommandClient) handleStatusConn(conn net.Conn) {
	for {
		var message StatusMessage
		err := binary.Read(conn, binary.BigEndian, &message)
		if err != nil {
			c.handler.Disconnected(err.Error())
			return
		}
		c.handler.WriteStatus(&message)
	}
}
