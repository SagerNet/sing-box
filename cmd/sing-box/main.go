package main

import (
	"os"

	"github.com/sagernet/sing-box/common/conf/mergers"
	"github.com/sagernet/sing-box/log"

	"github.com/spf13/cobra"
)

var (
	configPaths     []string
	configFormat    string
	configRecursive bool
	workingDir      string
	disableColor    bool
)

var mainCommand = &cobra.Command{
	Use:              "sing-box",
	PersistentPreRun: preRun,
}

func init() {
	mainCommand.PersistentFlags().StringArrayVarP(&configPaths, "config", "c", []string{"config.json"}, "set configuration file path")
	mainCommand.PersistentFlags().StringVarP(&configFormat, "format", "", string(mergers.FormatAuto), "load configuration directories recursively")
	mainCommand.PersistentFlags().BoolVarP(&configRecursive, "recursive", "r", false, "load configuration directories recursively")
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
