package main

import (
	"context"
	"os"

	"github.com/goccy/go-json"
	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/option"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var commandCheck = &cobra.Command{
	Use:   "check",
	Short: "Check configuration",
	Run:   checkConfiguration,
}

func checkConfiguration(cmd *cobra.Command, args []string) {
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		logrus.Fatal("read config: ", err)
	}
	var options option.Options
	err = json.Unmarshal(configContent, &options)
	if err != nil {
		logrus.Fatal("decode config: ", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err = box.NewService(ctx, options)
	if err != nil {
		logrus.Fatal("create service: ", err)
	}
	cancel()
}
