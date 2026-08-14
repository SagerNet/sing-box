package main

import (
	"github.com/sagernet/sing-box/daemon"

	"github.com/spf13/cobra"
)

var commandAPITailscaleTaildropDelete = &cobra.Command{
	Use:   "delete <name>...",
	Short: "Delete received files",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPITaildropDelete(args)
	},
}

func init() {
	commandAPITailscaleTaildrop.AddCommand(commandAPITailscaleTaildropDelete)
}

func runAPITaildropDelete(names []string) error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	endpointTag, err := resolveTailscaleEndpointTag(client)
	if err != nil {
		return err
	}
	for _, name := range names {
		_, err = client.DeleteTaildropFile(globalCtx, &daemon.DeleteTaildropFileRequest{
			EndpointTag: endpointTag,
			Name:        name,
		})
		if err != nil {
			return err
		}
	}
	return nil
}
