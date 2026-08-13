package main

import (
	"strings"
	"time"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/byteformats"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"

	"github.com/spf13/cobra"
)

var commandAPIConnectionShow = &cobra.Command{
	Use:   "show <id>",
	Short: "Print connection details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIConnectionShow(args[0])
	},
}

func init() {
	commandAPIConnection.AddCommand(commandAPIConnectionShow)
}

func runAPIConnectionShow(connectionID string) error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	connections, err := fetchConnections(client)
	if err != nil {
		return err
	}
	connection := common.Find(connections, func(it *daemon.Connection) bool {
		return it.GetId() == connectionID
	})
	if connection == nil {
		return E.New("connection not found: ", connectionID)
	}
	state := "open"
	if connection.GetClosedAt() != 0 {
		state = "closed"
	}
	var ipVersion string
	if connection.GetIpVersion() != 0 {
		ipVersion = F.ToString(connection.GetIpVersion())
	}
	inbound := connection.GetInboundType()
	if connection.GetInbound() != "" {
		inbound = connection.GetInboundType() + "/" + connection.GetInbound()
	}
	outbound := connection.GetOutbound()
	if outbound != "" && connection.GetOutboundType() != "" {
		outbound = F.ToString(outbound, " (", connection.GetOutboundType(), ")")
	}
	var block blockWriter
	block.addLine("ID", connection.GetId())
	block.addLine("State", state)
	block.addLine("Created", formatConnectionTime(connection.GetCreatedAt()))
	block.addLine("Closed", formatConnectionTime(connection.GetClosedAt()))
	block.addLine("Network", connection.GetNetwork())
	block.addLine("IP version", ipVersion)
	block.addLine("Protocol", connection.GetProtocol())
	block.addLine("Inbound", inbound)
	block.addLine("Source", connection.GetSource())
	block.addLine("Destination", connection.GetDestination())
	block.addLine("Domain", connection.GetDomain())
	block.addLine("User", connection.GetUser())
	block.addLine("Process", formatProcessInfo(connection.GetProcessInfo()))
	block.addLine("Rule", connection.GetRule())
	block.addLine("Outbound", outbound)
	block.addLine("Chain", strings.Join(connection.GetChainList(), " <- "))
	block.addLine("From outbound", connection.GetFromOutbound())
	block.addLine("Uplink", byteformats.FormatBytes(uint64(connection.GetUplinkTotal())))
	block.addLine("Downlink", byteformats.FormatBytes(uint64(connection.GetDownlinkTotal())))
	block.flush()
	return nil
}

func formatConnectionTime(timestamp int64) string {
	if timestamp == 0 {
		return ""
	}
	return time.UnixMilli(timestamp).Local().Format(time.RFC3339)
}

func formatProcessInfo(processInfo *daemon.ProcessInfo) string {
	if processInfo == nil {
		return ""
	}
	var process string
	if processInfo.GetProcessPath() != "" {
		process = processInfo.GetProcessPath()
	} else if len(processInfo.GetPackageNames()) > 0 {
		process = processInfo.GetPackageNames()[0]
	}
	if process == "" {
		if processInfo.GetUserId() != -1 {
			process = F.ToString(processInfo.GetUserId())
		}
	} else if processInfo.GetUserName() != "" {
		process = F.ToString(process, " (", processInfo.GetUserName(), ")")
	} else if processInfo.GetUserId() != -1 {
		process = F.ToString(process, " (", processInfo.GetUserId(), ")")
	}
	return process
}
