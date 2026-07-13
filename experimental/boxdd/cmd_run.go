package main

import (
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/sagernet/sing-box/experimental/libbox"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var (
	workingDirectory string
	socketPath       string
	listenAddress    string
)

var commandRun = &cobra.Command{
	Use:   "run",
	Short: "Run the daemon",
	Args:  cobra.NoArgs,
	Run: func(command *cobra.Command, args []string) {
		err := run()
		if err != nil {
			log.Fatal(E.Cause(err, "run daemon"))
		}
	},
}

func init() {
	commandRun.Flags().StringVarP(&workingDirectory, "working-directory", "D", "", "working directory")
	commandRun.Flags().StringVar(&socketPath, "socket", "", "listen on the specified unix domain socket path, or named pipe path on Windows")
	commandRun.Flags().StringVar(&listenAddress, "listen", "", "listen on the specified TCP address (development only)")
	mainCommand.AddCommand(commandRun)
}

func prepareWorkingDirectory() error {
	if workingDirectory == "" {
		return E.New("missing working directory")
	}
	absoluteWorkingDirectory, err := filepath.Abs(workingDirectory)
	if err != nil {
		return err
	}
	workingDirectory = absoluteWorkingDirectory
	err = preparePlatformWorkingDirectory()
	if err != nil {
		return err
	}
	err = os.Chdir(workingDirectory)
	if err != nil {
		return err
	}
	err = libbox.Setup(&libbox.SetupOptions{
		BasePath:          workingDirectory,
		WorkingPath:       workingDirectory,
		TempPath:          workingDirectory,
		CrashReportSource: "Daemon",
	})
	if err != nil {
		return err
	}
	libbox.PromoteOOMDraft()
	return nil
}

func run() error {
	handled, err := runService()
	if handled {
		return err
	}
	err = prepareWorkingDirectory()
	if err != nil {
		return err
	}
	d, err := newDaemon()
	if err != nil {
		return err
	}
	err = d.Start()
	if err != nil {
		return err
	}
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	<-signalChannel
	d.Close()
	return nil
}
