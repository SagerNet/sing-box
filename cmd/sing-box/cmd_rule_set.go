package main

import (
	"github.com/spf13/cobra"
)

var commandRuleSet = &cobra.Command{
	Use:   "rule-set",
	Short: "Manage rule sets",
}

func init() {
	mainCommand.AddCommand(commandRuleSet)
}
