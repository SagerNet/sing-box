package main

import (
	"github.com/sagernet/sing/common"

	"github.com/spf13/cobra"
)

var commandAPITailscaleExitNodeList = &cobra.Command{
	Use:   "list",
	Short: "List available Tailscale exit nodes",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPITailscaleExitNodeList()
	},
}

func init() {
	commandAPITailscaleExitNode.AddCommand(commandAPITailscaleExitNodeList)
}

func runAPITailscaleExitNodeList() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	endpoint, err := fetchTailscaleEndpoint(client)
	if err != nil {
		return err
	}
	selectedStableID := endpoint.GetExitNode().GetStableID()
	candidates := common.Filter(tailscalePeerEntries(endpoint), func(it tailscalePeerEntry) bool {
		return !it.self && it.peer.GetExitNodeOption()
	})
	sortTailscalePeerEntries(candidates)
	table := tableWriter{
		header:       []string{"DNS NAME", "IP", "ONLINE", "STATUS"},
		emptyMessage: "no exit nodes",
	}
	for _, entry := range candidates {
		var exitNodeStatus string
		if selectedStableID != "" && entry.peer.GetStableID() == selectedStableID {
			exitNodeStatus = "selected"
		}
		table.addRow(
			tailscalePeerName(entry.peer),
			tailscalePeerAddress(entry.peer),
			formatYesNo(entry.peer.GetOnline()),
			exitNodeStatus,
		)
	}
	table.flush()
	return nil
}
