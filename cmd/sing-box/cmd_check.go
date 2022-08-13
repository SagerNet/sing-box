package main

import (
	"context"
	"os"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/common/json"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var commandCheck = &cobra.Command{
	Use:   "check",
	Short: "Check configuration",
	Run: func(cmd *cobra.Command, args []string) {
		err := check()
		if err != nil {
			log.Fatal(err)
		}
	},
	Args: cobra.NoArgs,
}

func init() {
	mainCommand.AddCommand(commandCheck)
}

func check() error {
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		return E.Cause(err, "read config")
	}
	var options option.Options
	err = json.Unmarshal(configContent, &options)
	if err != nil {
		return E.Cause(err, "decode config")
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err = box.New(ctx, options)
	cancel()
	return err
}
