package main

import (
	"context"
	"os"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/common/json"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"

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
		log.Fatal("read config: ", err)
	}
	var options option.Options
	err = json.Unmarshal(configContent, &options)
	if err != nil {
		log.Fatal("decode config: ", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err = box.New(ctx, options)
	if err != nil {
		log.Fatal("create service: ", err)
	}
	cancel()
}
