package main

import (
	"github.com/spf13/cobra"
)

var commandAPITailscaleTaildropList = &cobra.Command{
	Use:   "list",
	Short: "List received files",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPITaildropList()
	},
}

func init() {
	commandAPITailscaleTaildrop.AddCommand(commandAPITailscaleTaildropList)
}

func runAPITaildropList() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	endpointTag, err := resolveTailscaleEndpointTag(client)
	if err != nil {
		return err
	}
	inbox, err := fetchTaildropInbox(client, endpointTag)
	if err != nil {
		return err
	}
	for _, file := range inbox.GetReceiving() {
		writeStderrLine("receiving " + file.GetName() + ": " + formatTaildropSize(file.GetReceivedBytes()) + " / " + formatTaildropSize(file.GetSize()))
	}
	table := tableWriter{
		header:       []string{"NAME", "SIZE", "FROM", "RECEIVED"},
		emptyMessage: "no received files",
	}
	for _, file := range inbox.GetFiles() {
		sender := file.GetSenderName()
		if sender == "" {
			sender = "-"
		}
		table.addRow(
			file.GetName(),
			formatTaildropSize(file.GetSize()),
			sender,
			formatTaildropModifiedAt(file.GetModifiedAt()),
		)
	}
	table.flush()
	return nil
}
