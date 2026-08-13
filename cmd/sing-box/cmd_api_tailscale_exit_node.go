package main

import (
	"os"

	F "github.com/sagernet/sing/common/format"

	"github.com/spf13/cobra"
)

var commandAPITailscaleExitNode = &cobra.Command{
	Use:   "exit-node",
	Short: "Print the current Tailscale exit node",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPITailscaleExitNode()
	},
}

func init() {
	commandAPITailscaleExitNode.PersistentFlags().StringVar(&commandAPITailscaleFlagEndpoint, "endpoint", "", commandAPITailscaleEndpointUsage)
	commandAPITailscale.AddCommand(commandAPITailscaleExitNode)
}

func runAPITailscaleExitNode() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	endpoint, err := fetchTailscaleEndpoint(client)
	if err != nil {
		return err
	}
	exitNode := endpoint.GetExitNode()
	if exitNode == nil {
		os.Stdout.WriteString("none\n")
		return nil
	}
	name := tailscalePeerName(exitNode)
	address := tailscalePeerAddress(exitNode)
	if address == "" {
		os.Stdout.WriteString(name + "\n")
		return nil
	}
	os.Stdout.WriteString(F.ToString(name, " (", address, ")", "\n"))
	return nil
}
