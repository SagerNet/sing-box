package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	runtimeDebug "runtime/debug"
	"syscall"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var commandRun = &cobra.Command{
	Use:   "run",
	Short: "Run service",
	Run: func(cmd *cobra.Command, args []string) {
		err := run()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	mainCommand.AddCommand(commandRun)
}

func readConfig() (option.Options, error) {
	var (
		configContent []byte
		err           error
	)
	if configPath == "stdin" {
		configContent, err = io.ReadAll(os.Stdin)
	} else {
		configContent, err = os.ReadFile(configPath)
	}
	if err != nil {
		return option.Options{}, E.Cause(err, "read config")
	}
	var options option.Options
	err = options.UnmarshalJSON(configContent)
	if err != nil {
		return option.Options{}, E.Cause(err, "decode config")
	}
	return options, nil
}

func create() (*box.Box, context.CancelFunc, error) {
	options, err := readConfig()
	if err != nil {
		return nil, nil, err
	}
	if disableColor {
		if options.Log == nil {
			options.Log = &option.LogOptions{}
		}
		options.Log.DisableColor = true
	}
	ctx, cancel := context.WithCancel(context.Background())
	instance, err := box.New(ctx, options)
	if err != nil {
		cancel()
		return nil, nil, E.Cause(err, "create service")
	}
	err = instance.Start()
	if err != nil {
		cancel()
		return nil, nil, E.Cause(err, "start service")
	}
	return instance, cancel, nil
}

func run() error {
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	for {
		instance, cancel, err := create()
		if err != nil {
			return err
		}
		runtimeDebug.FreeOSMemory()
		for {
			osSignal := <-osSignals
			if osSignal == syscall.SIGHUP {
				err = check()
				if err != nil {
					log.Error(E.Cause(err, "reload service"))
					continue
				}
			}
			cancel()
			instance.Close()
			if osSignal != syscall.SIGHUP {
				return nil
			}
			break
		}
	}
}
