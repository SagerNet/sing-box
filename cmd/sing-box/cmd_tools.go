package main

import (
	"context"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"

	"github.com/spf13/cobra"
)

var commandTools = &cobra.Command{
	Use:   "tools",
	Short: "Experimental tools",
}

func init() {
	mainCommand.AddCommand(commandTools)
}

func createPreStartedClient() (*box.Box, error) {
	options, err := readConfigAndMerge()
	if err != nil {
		return nil, err
	}
	if options.Log == nil {
		options.Log = &option.LogOptions{}
	}
	options.Log.Disabled = true
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

func createDialer(instance *box.Box, network string, outboundTag string) (N.Dialer, error) {
	if outboundTag == "" {
		outbound := instance.Router().DefaultOutbound(network)
		if outbound == nil {
			return nil, E.New("missing default outbound")
		}
		return outbound, nil
	} else {
		outbound, loaded := instance.Router().Outbound(outboundTag)
		if !loaded {
			return nil, E.New("outbound not found: ", outboundTag)
		}
		return outbound, nil
	}
}
