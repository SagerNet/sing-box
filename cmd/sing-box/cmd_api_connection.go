package main

import (
	"time"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing/common"

	"github.com/spf13/cobra"
)

var commandAPIConnection = &cobra.Command{
	Use:   "connection",
	Short: "Manage connections",
}

func init() {
	commandAPIRoot.AddCommand(commandAPIConnection)
}

func fetchConnections(client daemon.StartedServiceClient) ([]*daemon.Connection, error) {
	stream, err := client.SubscribeConnections(globalCtx, &daemon.SubscribeConnectionsRequest{Interval: int64(time.Second)})
	if err != nil {
		return nil, err
	}
	events, err := stream.Recv()
	if err != nil {
		return nil, err
	}
	connections := common.FilterNotNil(common.Map(events.GetEvents(), func(it *daemon.ConnectionEvent) *daemon.Connection {
		return it.GetConnection()
	}))
	common.SortBy(connections, func(it *daemon.Connection) int64 {
		return it.GetCreatedAt()
	})
	return connections, nil
}
