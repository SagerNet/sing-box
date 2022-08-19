package main

import (
	"context"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/log"

	"github.com/spf13/cobra"
)

var commandCheck = &cobra.Command{
	Use:   "check",
	Short: "Check configuration",
	Run: func(cmd *cobra.Command, args []string) {
		err := check()
		if err != nil {
			log.Fatal(err)
		}
	},
	Args: cobra.NoArgs,
}

func init() {
	mainCommand.AddCommand(commandCheck)
}

func check() error {
	options, err := readConfig()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err = box.New(ctx, options)
	cancel()
	return err
}
