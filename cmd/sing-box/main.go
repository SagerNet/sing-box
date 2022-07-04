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

func main() {
	command := &cobra.Command{
		Use:              "sing-box",
		PersistentPreRun: preRun,
	}
	command.PersistentFlags().StringVarP(&configPath, "config", "c", "config.json", "set configuration file path")
	command.PersistentFlags().StringVarP(&workingDir, "directory", "D", "", "set working directory")
	command.PersistentFlags().BoolVarP(&disableColor, "disable-color", "", false, "disable color output")
	command.AddCommand(commandRun)
	if err := command.Execute(); err != nil {
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
