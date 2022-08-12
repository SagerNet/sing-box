package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
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
	Run:   run,
}

func run(cmd *cobra.Command, args []string) {
	err := run0()
	if err != nil {
		log.Fatal(err)
	}
}

func run0() error {
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		return E.Cause(err, "read config")
	}
	var options option.Options
	err = json.Unmarshal(configContent, &options)
	if err != nil {
		return E.Cause(err, "decode config")
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
		return E.Cause(err, "create service")
	}
	err = instance.Start()
	if err != nil {
		cancel()
		return E.Cause(err, "start service")
	}
	if debug.Enabled {
		http.HandleFunc("/debug/close", func(writer http.ResponseWriter, request *http.Request) {
			cancel()
			instance.Close()
		})
	}
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)
	<-osSignals
	cancel()
	instance.Close()
	return nil
}
