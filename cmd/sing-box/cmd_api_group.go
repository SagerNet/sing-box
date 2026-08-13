package main

import (
	"github.com/sagernet/sing-box/daemon"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
)

var commandAPIGroup = &cobra.Command{
	Use:   "group",
	Short: "Manage outbound groups",
}

func init() {
	commandAPIRoot.AddCommand(commandAPIGroup)
}

func fetchGroups(client daemon.StartedServiceClient) ([]*daemon.Group, error) {
	stream, err := client.SubscribeGroups(globalCtx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	groups, err := stream.Recv()
	if err != nil {
		return nil, err
	}
	return groups.GetGroup(), nil
}
