package main

import (
	"github.com/sagernet/sing-box"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"

	"github.com/spf13/cobra"
)

var commandToolsFlagOutbound string

var commandTools = &cobra.Command{
	Use:   "tools",
	Short: "Experimental tools",
}

func init() {
	commandTools.PersistentFlags().StringVarP(&commandToolsFlagOutbound, "outbound", "o", "", "Use specified tag instead of default outbound")
	mainCommand.AddCommand(commandTools)
}

func createPreStartedClient() (*box.Box, error) {
	options, err := readConfigAndMerge()
	if err != nil {
		return nil, err
	}
	instance, err := box.New(box.Options{Options: options})
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
		return instance.Router().DefaultOutbound(N.NetworkName(network))
	} else {
		outbound, loaded := instance.Router().Outbound(outboundTag)
		if !loaded {
			return nil, E.New("outbound not found: ", outboundTag)
		}
		return outbound, nil
	}
}
