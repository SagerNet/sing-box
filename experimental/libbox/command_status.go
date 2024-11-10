package libbox

import (
	"encoding/binary"
	"net"
	"runtime"
	"time"

	"github.com/sagernet/sing-box/common/conntrack"
	"github.com/sagernet/sing-box/experimental/clashapi"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/memory"
)

type StatusMessage struct {
	Memory           int64
	Goroutines       int32
	ConnectionsIn    int32
	ConnectionsOut   int32
	TrafficAvailable bool
	Uplink           int64
	Downlink         int64
	UplinkTotal      int64
	DownlinkTotal    int64
}

func (s *CommandServer) readStatus() StatusMessage {
	var message StatusMessage
	message.Memory = int64(memory.Inuse())
	message.Goroutines = int32(runtime.NumGoroutine())
	message.ConnectionsOut = int32(conntrack.Count())

	if s.service != nil {
		message.TrafficAvailable = true
		trafficManager := s.service.clashServer.(*clashapi.Server).TrafficManager()
		message.UplinkTotal, message.DownlinkTotal = trafficManager.Total()
		message.ConnectionsIn = int32(trafficManager.ConnectionsLen())
	}

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
	status := s.readStatus()
	uploadTotal := status.UplinkTotal
	downloadTotal := status.DownlinkTotal
	for {
		err = binary.Write(conn, binary.BigEndian, status)
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		status = s.readStatus()
		upload := status.UplinkTotal - uploadTotal
		download := status.DownlinkTotal - downloadTotal
		uploadTotal = status.UplinkTotal
		downloadTotal = status.DownlinkTotal
		status.Uplink = upload
		status.Downlink = download
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
