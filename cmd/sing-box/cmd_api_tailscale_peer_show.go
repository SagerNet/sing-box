package main

import (
	"strings"
	"time"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-box/dns"
	F "github.com/sagernet/sing/common/format"

	"github.com/spf13/cobra"
)

const commandAPITailscalePeerLabelWidth = len("SSH host keys") + 3

var commandAPITailscalePeerShow = &cobra.Command{
	Use:   "show <peer>",
	Short: "Print Tailscale peer details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPITailscalePeerShow(args[0])
	},
}

func init() {
	commandAPITailscalePeer.AddCommand(commandAPITailscalePeerShow)
}

func runAPITailscalePeerShow(selector string) error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	endpoint, err := fetchTailscaleEndpoint(client)
	if err != nil {
		return err
	}
	entry, err := resolveTailscalePeer(tailscalePeerEntries(endpoint), selector)
	if err != nil {
		return err
	}
	peer := entry.peer
	exitNode := "no"
	if peer.GetExitNode() {
		exitNode = "in use"
	} else if peer.GetExitNodeOption() {
		exitNode = "offered"
	}
	var block blockWriter
	block.addLine("DNS name", dns.FqdnToDomain(peer.GetDnsName()))
	block.addLine("Host name", peer.GetHostName())
	block.addLine("Stable ID", peer.GetStableID())
	block.addLine("User", formatTailscaleUser(entry.group))
	block.addLine("OS", peer.GetOs())
	block.addLine("IPs", strings.Join(peer.GetTailscaleIPs(), ", "))
	block.addLine("Online", formatYesNo(peer.GetOnline()))
	block.addLine("Active", formatYesNo(peer.GetActive()))
	block.addLine("Expired", formatYesNo(peer.GetExpired()))
	block.addLine("Sharee node", formatYesNo(peer.GetShareeNode()))
	block.addLine("Exit node", exitNode)
	block.addLine("Rx", F.ToString(peer.GetRxBytes()))
	block.addLine("Tx", F.ToString(peer.GetTxBytes()))
	block.addLine("Key expiry", formatTailscaleTime(peer.GetKeyExpiry()))
	block.addLine("Last seen", formatTailscaleTime(peer.GetLastSeen()))
	block.addLine("SSH host keys", formatTailscaleSSHHostKeys(peer.GetSshHostKeys()))
	block.flush()
	return nil
}

func formatTailscaleUser(group *daemon.TailscaleUserGroup) string {
	loginName := group.GetLoginName()
	displayName := group.GetDisplayName()
	if displayName == "" || displayName == loginName {
		return loginName
	}
	if loginName == "" {
		return displayName
	}
	return F.ToString(loginName, " (", displayName, ")")
}

func formatTailscaleTime(timestamp int64) string {
	if timestamp == 0 {
		return ""
	}
	return time.Unix(timestamp, 0).Local().Format(time.RFC3339)
}

func formatTailscaleSSHHostKeys(hostKeys []string) string {
	if len(hostKeys) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString(F.ToString(len(hostKeys)))
	for _, hostKey := range hostKeys {
		builder.WriteString("\n")
		builder.WriteString(strings.Repeat(" ", commandAPITailscalePeerLabelWidth))
		builder.WriteString(hostKey)
	}
	return builder.String()
}
