package main

import (
	"github.com/sagernet/sing-box/daemon"

	"github.com/spf13/cobra"
)

var commandAPITailscaleLogout = &cobra.Command{
	Use:   "logout",
	Short: "Log out of the Tailscale network",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPITailscaleLogout()
	},
}

func init() {
	commandAPITailscaleLogout.Flags().StringVar(&commandAPITailscaleFlagEndpoint, "endpoint", "", commandAPITailscaleEndpointUsage)
	commandAPITailscale.AddCommand(commandAPITailscaleLogout)
}

func runAPITailscaleLogout() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	endpointTag, err := resolveTailscaleEndpointTag(client)
	if err != nil {
		return err
	}
	_, err = client.TailscaleLogout(globalCtx, &daemon.TailscaleLogoutRequest{
		EndpointTag: endpointTag,
	})
	return err
}
