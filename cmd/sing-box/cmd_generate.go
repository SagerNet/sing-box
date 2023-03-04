package main

import (
	"github.com/sagernet/sing-box/cmd/sing-box/internal/generate"

	"github.com/spf13/cobra"
)

var commandGenerate = &cobra.Command{
	Use:   "generate",
	Short: "Generate things",
}

func init() {
	commandGenerate.AddCommand(generate.CommandGenerateRandom)
	commandGenerate.AddCommand(generate.CommandGenerateUUID)
	commandGenerate.AddCommand(generate.CommandGenerateReality)
	mainCommand.AddCommand(commandGenerate)
}
