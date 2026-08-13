package main

import (
	"os"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var commandAPITailscaleStatus = &cobra.Command{
	Use:   "status",
	Short: "Print the status of Tailscale endpoints",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPITailscaleStatus()
	},
}

func init() {
	commandAPITailscale.AddCommand(commandAPITailscaleStatus)
}

func runAPITailscaleStatus() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	endpoints, err := fetchTailscaleStatus(client)
	if err != nil {
		return err
	}
	if len(endpoints) == 0 {
		return E.New("no tailscale endpoint found")
	}
	for index, endpoint := range endpoints {
		if index > 0 {
			os.Stdout.WriteString("\n")
		}
		os.Stdout.WriteString(endpoint.GetEndpointTag() + "\n")
		var block blockWriter
		block.addLine("  State", endpoint.GetBackendState())
		if endpoint.GetNetworkName() != "" {
			block.addLine("  Network", endpoint.GetNetworkName())
		}
		if endpoint.GetKeyAuth() {
			block.addLine("  Auth", "auth key")
		}
		if endpoint.GetAuthURL() != "" {
			block.addLine("  Log in", endpoint.GetAuthURL())
		}
		block.flush()
	}
	return nil
}
