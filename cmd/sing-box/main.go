package main

import (
	"os"

	"github.com/sagernet/sing-box/log"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	logrus.StandardLogger().SetLevel(logrus.TraceLevel)
	logrus.StandardLogger().SetFormatter(&log.LogrusTextFormatter{})
}

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

	mainCommand.AddCommand(commandRun)
	mainCommand.AddCommand(commandCheck)
	mainCommand.AddCommand(commandFormat)
	mainCommand.AddCommand(commandVersion)
}

func main() {
	if err := mainCommand.Execute(); err != nil {
		logrus.Fatal(err)
	}
}

func preRun(cmd *cobra.Command, args []string) {
	if disableColor {
		logrus.StandardLogger().SetFormatter(&log.LogrusTextFormatter{DisableColors: true})
	}
	if workingDir != "" {
		if err := os.Chdir(workingDir); err != nil {
			logrus.Fatal(err)
		}
	}
}
