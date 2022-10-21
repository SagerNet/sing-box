package main

import (
	"os"

	_ "github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/log"

	"github.com/spf13/cobra"
)

var (
	configPaths     []string
	configRecursive bool
	workingDir      string
	disableColor    bool
)

var mainCommand = &cobra.Command{
	Use:              "sing-box",
	PersistentPreRun: preRun,
}

func init() {
	mainCommand.PersistentFlags().StringArrayVarP(&configPaths, "config", "c", []string{"config.json"}, "set configuration files / directories")
	mainCommand.PersistentFlags().BoolVarP(&configRecursive, "config-recursive", "r", false, "load configuration directories recursively")
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
