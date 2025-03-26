package main

import (
	"github.com/spf13/cobra"
)

var commandToolsFlagOutbound string

var commandTools = &cobra.Command{
	Use:   "tools",
	Short: "Experimental tools",
}

func init() {
	mainCommand.AddCommand(commandTools)
}
