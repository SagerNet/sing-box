package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/goccy/go-json"
	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/option"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var commandRun = &cobra.Command{
	Use:   "run",
	Short: "run service",
	Run:   run,
}

func run(cmd *cobra.Command, args []string) {
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		logrus.Fatal("read config: ", err)
	}
	var options option.Options
	err = json.Unmarshal(configContent, &options)
	if err != nil {
		logrus.Fatal("decode config: ", err)
	}
	if disableColor {
		if options.Log == nil {
			options.Log = &option.LogOption{}
		}
		options.Log.DisableColor = true
	}

	ctx, cancel := context.WithCancel(context.Background())
	service, err := box.NewService(ctx, options)
	if err != nil {
		logrus.Fatal("create service: ", err)
	}
	err = service.Start()
	if err != nil {
		logrus.Fatal("start service: ", err)
	}
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)
	<-osSignals
	cancel()
	service.Close()
}
