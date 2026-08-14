package main

import (
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var commandAPITailscaleTaildropTargets = &cobra.Command{
	Use:   "targets",
	Short: "List peers that can receive files",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPITaildropTargets()
	},
}

func init() {
	commandAPITailscaleTaildrop.AddCommand(commandAPITailscaleTaildropTargets)
}

func runAPITaildropTargets() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	endpoint, err := fetchTailscaleEndpoint(client)
	if err != nil {
		return err
	}
	if !endpoint.GetCanShareFiles() {
		return E.New("file sharing not enabled by Tailscale admin")
	}
	entries := tailscalePeerEntries(endpoint)
	if len(entries) > 0 && entries[0].self {
		entries = entries[1:]
	}
	sortTailscalePeerEntries(entries)
	table := tableWriter{
		header:       []string{"DNS NAME", "IP", "ONLINE"},
		emptyMessage: "no file targets",
	}
	for _, entry := range entries {
		if !entry.peer.GetCanReceiveFiles() {
			continue
		}
		table.addRow(
			tailscalePeerName(entry.peer),
			tailscalePeerAddress(entry.peer),
			formatYesNo(entry.peer.GetOnline()),
		)
	}
	table.flush()
	return nil
}
