package main

import (
	"context"

	box "github.com/sagernet/sing-box"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var commandTools = &cobra.Command{
	Use:   "tools",
	Short: "experimental tools",
}

func init() {
	mainCommand.AddCommand(commandTools)
}

func createPreStartedClient() (*box.Box, error) {
	options, err := readConfigAndMerge()
	if err != nil {
		return nil, err
	}
	instance, err := box.New(context.Background(), options, nil)
	if err != nil {
		return nil, E.Cause(err, "create service")
	}
	err = instance.PreStart()
	if err != nil {
		return nil, E.Cause(err, "start service")
	}
	return instance, nil
}
