package main

import (
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var commandAPIGroupShow = &cobra.Command{
	Use:   "show <group>",
	Short: "Show an outbound group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIGroupShow(args[0])
	},
}

func init() {
	commandAPIGroup.AddCommand(commandAPIGroupShow)
}

func runAPIGroupShow(groupTag string) error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	groups, err := fetchGroups(client)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group.GetTag() != groupTag {
			continue
		}
		block := blockWriter{}
		block.addLine("Tag", group.GetTag())
		block.addLine("Type", group.GetType())
		block.addLine("Selected", group.GetSelected())
		block.flush()
		table := tableWriter{
			header: []string{"TAG", "TYPE", "DELAY"},
		}
		for _, item := range group.GetItems() {
			table.addRow(item.GetTag(), item.GetType(), formatDelay(item.GetUrlTestDelay()))
		}
		table.flush()
		return nil
	}
	return E.New("group not found: ", groupTag)
}
