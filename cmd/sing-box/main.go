package main

import (
	"os"

	"github.com/sagernet/sing-box/log"

	"github.com/spf13/cobra"
)

var (
	configPath   string
	workingDir   string
	disableColor bool
)

var mainCommand = &cobra.Command{
	Use:              "sing-box",
	PersistentPreRun: preRun,
}

func init() {
	mainCommand.PersistentFlags().StringVarP(&configPath, "config", "c", "config.json", "set configuration file path")
	mainCommand.PersistentFlags().StringVarP(&workingDir, "directory", "D", "", "set working directory")
	mainCommand.PersistentFlags().BoolVarP(&disableColor, "disable-color", "", false, "disable color output")
}

func main() {
	if err := mainCommand.Execute(); err != nil {
		log.Fatal(err)
	}
}

func preRun(cmd *cobra.Command, args []string) {
	if workingDir != "" {
		if err := os.Chdir(workingDir); err != nil {
			log.Fatal(err)
		}
	}
}
