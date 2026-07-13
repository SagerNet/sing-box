package main

import (
	"os"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var commandServiceFlagWorkingDirectory string

var commandService = &cobra.Command{
	Use:   "service",
	Short: "Manage the system service",
}

var commandServiceStart = &cobra.Command{
	Use:   "start",
	Short: "Start the system service",
	Args:  cobra.NoArgs,
	Run: func(command *cobra.Command, args []string) {
		err := serviceStart()
		if err != nil {
			log.Fatal(E.Cause(err, "start service"))
		}
	},
}

var commandServiceStop = &cobra.Command{
	Use:   "stop",
	Short: "Stop the system service",
	Args:  cobra.NoArgs,
	Run: func(command *cobra.Command, args []string) {
		err := serviceStop()
		if err != nil {
			log.Fatal(E.Cause(err, "stop service"))
		}
	},
}

var commandServiceStatus = &cobra.Command{
	Use:   "status",
	Short: "Print the system service status",
	Args:  cobra.NoArgs,
	Run: func(command *cobra.Command, args []string) {
		status, err := serviceStatus()
		if err != nil {
			log.Fatal(E.Cause(err, "query service status"))
		}
		os.Stdout.WriteString(status.description + "\n")
		os.Exit(status.exitCode)
	},
}

type serviceStatusResult struct {
	exitCode    int
	description string
}

func init() {
	commandService.PersistentFlags().StringVarP(&commandServiceFlagWorkingDirectory, "working-directory", "D", defaultServiceWorkingDirectory, "daemon working directory")
	commandService.AddCommand(commandServiceStart)
	commandService.AddCommand(commandServiceStop)
	commandService.AddCommand(commandServiceStatus)
	addPlatformServiceCommands()
	mainCommand.AddCommand(commandService)
}
