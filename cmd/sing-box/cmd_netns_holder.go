package main

import (
	"github.com/sagernet/sing-box/common/netns"

	"github.com/spf13/cobra"
)

var commandNetnsHolder = &cobra.Command{
	Use:    "netns-holder",
	Args:   cobra.NoArgs,
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		netns.Hold()
	},
}

func init() {
	mainCommand.AddCommand(commandNetnsHolder)
}
