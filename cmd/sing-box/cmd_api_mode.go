package main

import (
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
)

var commandAPIMode = &cobra.Command{
	Use:   "mode",
	Short: "Print the current clash mode",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIMode()
	},
}

func init() {
	commandAPIRoot.AddCommand(commandAPIMode)
}

func runAPIMode() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	modeStatus, err := client.GetClashModeStatus(globalCtx, &emptypb.Empty{})
	if err != nil {
		return err
	}
	currentMode := modeStatus.GetCurrentMode()
	if currentMode == "" {
		currentMode = "-"
	}
	os.Stdout.WriteString(currentMode + "\n")
	return nil
}
