package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/common/json"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"

	"github.com/spf13/cobra"
)

var commandRun = &cobra.Command{
	Use:   "run",
	Short: "Run service",
	Run:   run,
}

func run(cmd *cobra.Command, args []string) {
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatal("read config: ", err)
	}
	var options option.Options
	err = json.Unmarshal(configContent, &options)
	if err != nil {
		log.Fatal("decode config: ", err)
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
		log.Fatal("create service: ", err)
	}
	err = instance.Start()
	if err != nil {
		log.Fatal("start service: ", err)
	}
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)
	<-osSignals
	cancel()
	instance.Close()
}
