package main

import (
	"github.com/sagernet/sing-box/daemon"

	"github.com/spf13/cobra"
)

var commandAPIModeSet = &cobra.Command{
	Use:   "set <mode>",
	Short: "Set the clash mode",
	Long:  "Set the clash mode.\n\nThe value is not validated against the mode list: setting an unknown mode reports success.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIModeSet(args[0])
	},
}

func init() {
	commandAPIMode.AddCommand(commandAPIModeSet)
}

func runAPIModeSet(mode string) error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	_, err = client.SetClashMode(globalCtx, &daemon.ClashMode{Mode: mode})
	return err
}
