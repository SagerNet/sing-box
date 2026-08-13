package main

import (
	"github.com/sagernet/sing-box/daemon"

	"github.com/spf13/cobra"
)

var commandAPIGroupSelect = &cobra.Command{
	Use:   "select <group> <outbound>",
	Short: "Select an outbound in a group",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIGroupSelect(args[0], args[1])
	},
}

func init() {
	commandAPIGroup.AddCommand(commandAPIGroupSelect)
}

func runAPIGroupSelect(groupTag string, outboundTag string) error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	_, err = client.SelectOutbound(globalCtx, &daemon.SelectOutboundRequest{
		GroupTag:    groupTag,
		OutboundTag: outboundTag,
	})
	return err
}
