package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	logrus.StandardLogger().SetLevel(logrus.TraceLevel)
	logrus.StandardLogger().Formatter.(*logrus.TextFormatter).ForceColors = true
}

var configPath string

func main() {
	command := &cobra.Command{
		Use: "sing-box",
		Run: run,
	}
	command.Flags().StringVarP(&configPath, "config", "c", "config.json", "set configuration file path")
	if err := command.Execute(); err != nil {
		logrus.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) {
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		logrus.Fatal("read config: ", err)
	}
	var boxConfig config.Config
	err = json.Unmarshal(configContent, &boxConfig)
	if err != nil {
		logrus.Fatal("parse config: ", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	service, err := box.NewService(ctx, &boxConfig)
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
