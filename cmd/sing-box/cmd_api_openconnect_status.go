package main

import (
	"context"
	"os"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/daemon"
	F "github.com/sagernet/sing/common/format"

	"github.com/spf13/cobra"
)

var commandAPIOpenConnectStatus = &cobra.Command{
	Use:   "status",
	Short: "Print OpenConnect endpoint status",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIOpenConnectStatus()
	},
}

func init() {
	commandAPIOpenConnect.AddCommand(commandAPIOpenConnectStatus)
}

func runAPIOpenConnectStatus() error {
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
	if len(endpoints) == 0 {
		writeStderrLine("no openconnect endpoint is configured")
		return nil
	}
	for index, endpointStatus := range endpoints {
		if index > 0 {
			os.Stdout.WriteString("\n")
		}
		writeOpenConnectStatusBlock(endpointStatus)
	}
	return nil
}

func writeOpenConnectStatusBlock(endpointStatus *daemon.OpenConnectEndpointStatus) {
	var block blockWriter
	block.addLine("Endpoint", endpointStatus.GetEndpointTag())
	block.addLine("State", endpointStatus.GetState())
	challenge := endpointStatus.GetAuthChallenge()
	tunnelInfo := endpointStatus.GetTunnelInfo()
	switch {
	case challenge != nil:
		block.addLine("Challenge", openConnectChallengeSummary(challenge))
		if challenge.GetMessage() != "" {
			block.addLine("Message", challenge.GetMessage())
		}
		if challenge.GetError() != "" {
			block.addLine("Error", challenge.GetError())
		}
	case tunnelInfo != nil:
		block.addLine("Server", tunnelInfo.GetServer())
		block.addLine("Flavor", tunnelInfo.GetFlavor())
		block.addLine("Transport", tunnelInfo.GetTransport())
		if len(tunnelInfo.GetIpv4()) > 0 {
			block.addLine("IPv4", strings.Join(tunnelInfo.GetIpv4(), ", "))
		}
		if len(tunnelInfo.GetIpv6()) > 0 {
			block.addLine("IPv6", strings.Join(tunnelInfo.GetIpv6(), ", "))
		}
		if len(tunnelInfo.GetDns()) > 0 {
			block.addLine("DNS", strings.Join(tunnelInfo.GetDns(), ", "))
		}
		if tunnelInfo.GetMtu() > 0 {
			block.addLine("MTU", F.ToString(tunnelInfo.GetMtu()))
		}
		block.addLine("Connected since", formatVPNConnectedSince(tunnelInfo.GetConnectedSince()))
	case endpointStatus.GetState() == adapter.OpenConnectStateError:
		block.addLine("Error", endpointStatus.GetError())
	}
	block.flush()
	if challenge != nil {
		writeStderrLine("")
		writeStderrLine(`run "sing-box api openconnect auth" to answer`)
	}
}
