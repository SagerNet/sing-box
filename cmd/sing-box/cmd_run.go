package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/signal"
	runtimeDebug "runtime/debug"
	"syscall"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/common/json"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/debug"
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
	err = json.Unmarshal(configContent, &options)
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
	instance, cancel, err := create()
	if err != nil {
		return err
	}
	if debug.Enabled {
		http.HandleFunc("/debug/close", func(writer http.ResponseWriter, request *http.Request) {
			cancel()
			instance.Close()
		})
	}
	runtimeDebug.FreeOSMemory()
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)
	<-osSignals
	cancel()
	instance.Close()
	return nil
}
