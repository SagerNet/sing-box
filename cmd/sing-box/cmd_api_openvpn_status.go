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

var commandAPIOpenVPNStatus = &cobra.Command{
	Use:   "status",
	Short: "Print OpenVPN endpoint status",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIOpenVPNStatus()
	},
}

func init() {
	commandAPIOpenVPN.AddCommand(commandAPIOpenVPNStatus)
}

func runAPIOpenVPNStatus() error {
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
	if len(endpoints) == 0 {
		writeStderrLine("no openvpn endpoint is configured")
		return nil
	}
	for index, endpointStatus := range endpoints {
		if index > 0 {
			os.Stdout.WriteString("\n")
		}
		writeOpenVPNStatusBlock(endpointStatus)
	}
	return nil
}

func writeOpenVPNStatusBlock(endpointStatus *daemon.OpenVPNEndpointStatus) {
	var block blockWriter
	block.addLine("Endpoint", endpointStatus.GetEndpointTag())
	block.addLine("State", endpointStatus.GetState())
	challenge := endpointStatus.GetChallenge()
	tunnelInfo := endpointStatus.GetTunnelInfo()
	switch {
	case challenge != nil:
		block.addLine("Challenge", openVPNChallengeSummary(challenge))
		if challenge.GetMessage() != "" {
			block.addLine("Message", challenge.GetMessage())
		}
		if challenge.GetUrl() != "" {
			block.addLine("URL", challenge.GetUrl())
		}
		if challenge.GetDeadline() != 0 {
			block.addLine("Deadline", "in "+formatAuthDeadline(challenge.GetDeadline()))
		}
		if challenge.GetPreviousError() != "" {
			block.addLine("Error", challenge.GetPreviousError())
		}
	case tunnelInfo != nil:
		block.addLine("Server", tunnelInfo.GetServer())
		block.addLine("Network", tunnelInfo.GetNetwork())
		block.addLine("Cipher", tunnelInfo.GetCipher())
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
	case endpointStatus.GetState() == adapter.OpenVPNStateError:
		block.addLine("Error", endpointStatus.GetError())
	}
	block.flush()
	if challenge != nil {
		writeStderrLine("")
		writeStderrLine(`run "sing-box api openvpn auth" to continue`)
	}
}
