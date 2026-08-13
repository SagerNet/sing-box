package main

import (
	"github.com/sagernet/sing-box/daemon"

	"github.com/spf13/cobra"
)

var commandAPITailscaleExitNodeSet = &cobra.Command{
	Use:   "set <peer>",
	Short: "Use a Tailscale peer as exit node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPITailscaleExitNodeSet(args[0])
	},
}

func init() {
	commandAPITailscaleExitNode.AddCommand(commandAPITailscaleExitNodeSet)
}

func runAPITailscaleExitNodeSet(selector string) error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	endpoint, err := fetchTailscaleEndpoint(client)
	if err != nil {
		return err
	}
	entry, err := resolveTailscalePeer(tailscalePeerEntries(endpoint), selector)
	if err != nil {
		return err
	}
	_, err = client.SetTailscaleExitNode(globalCtx, &daemon.SetTailscaleExitNodeRequest{
		EndpointTag: endpoint.GetEndpointTag(),
		StableID:    entry.peer.GetStableID(),
	})
	return err
}
