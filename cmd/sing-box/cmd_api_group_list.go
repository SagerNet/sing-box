package main

import (
	"github.com/spf13/cobra"
)

var commandAPIGroupList = &cobra.Command{
	Use:   "list",
	Short: "List outbound groups",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIGroupList()
	},
}

func init() {
	commandAPIGroup.AddCommand(commandAPIGroupList)
}

func runAPIGroupList() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	groups, err := fetchGroups(client)
	if err != nil {
		return err
	}
	table := tableWriter{
		header:       []string{"TAG", "TYPE", "SELECTED"},
		emptyMessage: "no groups",
	}
	for _, group := range groups {
		table.addRow(group.GetTag(), group.GetType(), group.GetSelected())
	}
	table.flush()
	return nil
}
