package main

import (
	"context"
	"strings"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
)

const commandAPITailscaleEndpointUsage = "Tailscale endpoint tag (default: the only Tailscale endpoint)"

var commandAPITailscaleFlagEndpoint string

var commandAPITailscale = &cobra.Command{
	Use:   "tailscale",
	Short: "Manage Tailscale endpoints",
}

func init() {
	commandAPIRoot.AddCommand(commandAPITailscale)
}

func fetchTailscaleStatus(client daemon.StartedServiceClient) ([]*daemon.TailscaleEndpointStatus, error) {
	ctx, cancel := context.WithCancel(globalCtx)
	defer cancel()
	stream, err := client.SubscribeTailscaleStatus(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	update, err := stream.Recv()
	if err != nil {
		return nil, err
	}
	endpoints := update.GetEndpoints()
	common.SortBy(endpoints, func(it *daemon.TailscaleEndpointStatus) string {
		return it.GetEndpointTag()
	})
	return endpoints, nil
}

func resolveTailscaleEndpointStatus(endpoints []*daemon.TailscaleEndpointStatus) (*daemon.TailscaleEndpointStatus, error) {
	if len(endpoints) == 0 {
		return nil, E.New("no tailscale endpoint found")
	}
	if commandAPITailscaleFlagEndpoint != "" {
		endpoint := common.Find(endpoints, func(it *daemon.TailscaleEndpointStatus) bool {
			return it.GetEndpointTag() == commandAPITailscaleFlagEndpoint
		})
		if endpoint == nil {
			return nil, E.New("unknown tailscale endpoint: ", commandAPITailscaleFlagEndpoint, "\nknown endpoints:\n", formatTailscaleEndpointTags(endpoints))
		}
		return endpoint, nil
	}
	if len(endpoints) > 1 {
		return nil, E.New("multiple tailscale endpoints, use --endpoint to select one:\n", formatTailscaleEndpointTags(endpoints))
	}
	return endpoints[0], nil
}

func fetchTailscaleEndpoint(client daemon.StartedServiceClient) (*daemon.TailscaleEndpointStatus, error) {
	endpoints, err := fetchTailscaleStatus(client)
	if err != nil {
		return nil, err
	}
	return resolveTailscaleEndpointStatus(endpoints)
}

func resolveTailscaleEndpointTag(client daemon.StartedServiceClient) (string, error) {
	if commandAPITailscaleFlagEndpoint != "" {
		return commandAPITailscaleFlagEndpoint, nil
	}
	endpoint, err := fetchTailscaleEndpoint(client)
	if err != nil {
		return "", err
	}
	return endpoint.GetEndpointTag(), nil
}

func formatTailscaleEndpointTags(endpoints []*daemon.TailscaleEndpointStatus) string {
	return strings.Join(common.Map(endpoints, func(it *daemon.TailscaleEndpointStatus) string {
		return "  " + it.GetEndpointTag()
	}), "\n")
}
