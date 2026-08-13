package main

import (
	"context"
	"os"

	"github.com/sagernet/sing-box/daemon"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var commandAPIOpenVPNCancelFlagEndpoint string

var commandAPIOpenVPNCancel = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel the pending OpenVPN challenge and stop the client",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIOpenVPNCancel()
	},
}

func init() {
	commandAPIOpenVPNCancel.Flags().StringVar(&commandAPIOpenVPNCancelFlagEndpoint, "endpoint", "", "OpenVPN endpoint tag (default: the only configured endpoint)")
	commandAPIOpenVPN.AddCommand(commandAPIOpenVPNCancel)
}

func runAPIOpenVPNCancel() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	ctx, cancel := context.WithCancel(globalCtx)
	defer cancel()
	_, endpoints, err := subscribeOpenVPNStatus(ctx, client)
	if err != nil {
		return err
	}
	endpointStatus, err := resolveVPNEndpoint(endpoints, commandAPIOpenVPNCancelFlagEndpoint, "openvpn")
	if err != nil {
		return err
	}
	endpointTag := endpointStatus.GetEndpointTag()
	challenge := endpointStatus.GetChallenge()
	if challenge == nil {
		return E.New("no pending authentication challenge on ", endpointTag)
	}
	_, err = client.CancelOpenVPNChallenge(ctx, &daemon.OpenVPNChallengeCancel{
		EndpointTag: endpointTag,
		ChallengeID: challenge.GetId(),
	})
	if err != nil {
		return err
	}
	// sing-openvpn treats a canceled challenge as terminal: unlike OpenConnect, the client does not
	// reconnect afterwards.
	os.Stdout.WriteString(endpointTag + ": authentication challenge canceled; the client has stopped\n")
	return nil
}
