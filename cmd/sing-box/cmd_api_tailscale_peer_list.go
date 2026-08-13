package main

import (
	"github.com/spf13/cobra"
)

var commandAPITailscalePeerList = &cobra.Command{
	Use:   "list",
	Short: "List Tailscale peers",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPITailscalePeerList()
	},
}

func init() {
	commandAPITailscalePeer.AddCommand(commandAPITailscalePeerList)
}

func runAPITailscalePeerList() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	endpoint, err := fetchTailscaleEndpoint(client)
	if err != nil {
		return err
	}
	entries := tailscalePeerEntries(endpoint)
	sortableEntries := entries
	if len(sortableEntries) > 0 && sortableEntries[0].self {
		sortableEntries = sortableEntries[1:]
	}
	sortTailscalePeerEntries(sortableEntries)
	table := tableWriter{
		header:       []string{"DNS NAME", "IP", "ONLINE"},
		emptyMessage: "no peers",
	}
	for _, entry := range entries {
		table.addRow(
			tailscalePeerName(entry.peer),
			tailscalePeerAddress(entry.peer),
			formatYesNo(entry.peer.GetOnline()),
		)
	}
	table.flush()
	return nil
}
