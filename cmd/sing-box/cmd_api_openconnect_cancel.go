package main

import (
	"context"
	"os"

	"github.com/sagernet/sing-box/daemon"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var commandAPIOpenConnectCancelFlagEndpoint string

var commandAPIOpenConnectCancel = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel the pending OpenConnect authentication challenge",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIOpenConnectCancel()
	},
}

func init() {
	commandAPIOpenConnectCancel.Flags().StringVar(&commandAPIOpenConnectCancelFlagEndpoint, "endpoint", "", "OpenConnect endpoint tag (default: the only configured endpoint)")
	commandAPIOpenConnect.AddCommand(commandAPIOpenConnectCancel)
}

func runAPIOpenConnectCancel() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	ctx, cancel := context.WithCancel(globalCtx)
	defer cancel()
	_, endpoints, err := subscribeOpenConnectStatus(ctx, client)
	if err != nil {
		return err
	}
	endpointStatus, err := resolveVPNEndpoint(endpoints, commandAPIOpenConnectCancelFlagEndpoint, "openconnect")
	if err != nil {
		return err
	}
	endpointTag := endpointStatus.GetEndpointTag()
	challenge := endpointStatus.GetAuthChallenge()
	if challenge == nil {
		return E.New("no pending authentication challenge on ", endpointTag)
	}
	_, err = client.CancelOpenConnectAuthChallenge(ctx, &daemon.OpenConnectAuthChallengeCancel{
		EndpointTag: endpointTag,
		ChallengeID: challenge.GetId(),
	})
	if err != nil {
		return err
	}
	os.Stdout.WriteString(endpointTag + ": authentication challenge canceled; the client will restart authentication\n")
	return nil
}
