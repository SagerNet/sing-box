package main

import (
	"github.com/sagernet/sing-box/daemon"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
)

var commandAPIConnectionCloseFlagAll bool

var commandAPIConnectionClose = &cobra.Command{
	Use:   "close <id>",
	Short: "Close connections",
	Long:  "Close connections.\n\nThe id must be a full UUID; the service reports success for an unknown or already closed connection.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIConnectionClose(args)
	},
}

func init() {
	commandAPIConnectionClose.Flags().BoolVar(&commandAPIConnectionCloseFlagAll, "all", false, "Close all connections")
	commandAPIConnection.AddCommand(commandAPIConnectionClose)
}

func runAPIConnectionClose(args []string) error {
	if commandAPIConnectionCloseFlagAll {
		if len(args) > 0 {
			return E.New("--all takes no connection id")
		}
	} else if len(args) == 0 {
		return E.New("missing connection id")
	}
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	if commandAPIConnectionCloseFlagAll {
		_, err = client.CloseAllConnections(globalCtx, &emptypb.Empty{})
	} else {
		_, err = client.CloseConnection(globalCtx, &daemon.CloseConnectionRequest{Id: args[0]})
	}
	return err
}
