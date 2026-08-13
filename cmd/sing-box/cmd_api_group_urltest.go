package main

import (
	"github.com/sagernet/sing-box/daemon"

	"github.com/spf13/cobra"
)

var commandAPIGroupURLTest = &cobra.Command{
	Use:   "urltest <group>",
	Short: "Start a URL test",
	Long:  "Start a URL test.\n\nThe tests are only spawned: results appear in `outbounds --group <group>` a few seconds later.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIGroupURLTest(args[0])
	},
}

func init() {
	commandAPIGroup.AddCommand(commandAPIGroupURLTest)
}

func runAPIGroupURLTest(groupTag string) error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	_, err = client.URLTest(globalCtx, &daemon.URLTestRequest{OutboundTag: groupTag})
	return err
}
