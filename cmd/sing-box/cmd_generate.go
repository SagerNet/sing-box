package main

import (
	"github.com/spf13/cobra"

	"github.com/sagernet/sing-box/cmd/sing-box/internal/generate"
)

var commandGenerate = &cobra.Command{
	Use:   "generate",
	Short: "Generate things",
}

func init() {
	commandGenerate.AddCommand(generate.CommandGeneratePSK)
	commandGenerate.AddCommand(generate.CommandGenerateUUID)
	commandGenerate.AddCommand(generate.CommandGenerateX25519)
	mainCommand.AddCommand(commandGenerate)
}
