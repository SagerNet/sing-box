package main

import (
	"github.com/sagernet/sing-box/daemon"

	"github.com/spf13/cobra"
)

var commandAPITailscaleExitNodeClear = &cobra.Command{
	Use:   "clear",
	Short: "Stop using a Tailscale exit node",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPITailscaleExitNodeClear()
	},
}

func init() {
	commandAPITailscaleExitNode.AddCommand(commandAPITailscaleExitNodeClear)
}

func runAPITailscaleExitNodeClear() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	endpointTag, err := resolveTailscaleEndpointTag(client)
	if err != nil {
		return err
	}
	_, err = client.SetTailscaleExitNode(globalCtx, &daemon.SetTailscaleExitNodeRequest{
		EndpointTag: endpointTag,
	})
	return err
}
