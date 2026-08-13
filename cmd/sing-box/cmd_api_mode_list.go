package main

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
)

var commandAPIModeList = &cobra.Command{
	Use:   "list",
	Short: "List clash modes",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIModeList()
	},
}

func init() {
	commandAPIMode.AddCommand(commandAPIModeList)
}

func runAPIModeList() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	modeStatus, err := client.GetClashModeStatus(globalCtx, &emptypb.Empty{})
	if err != nil {
		return err
	}
	if len(modeStatus.GetModeList()) == 0 {
		writeStderrLine("no clash modes")
		return nil
	}
	var output strings.Builder
	for _, mode := range modeStatus.GetModeList() {
		output.WriteString(mode)
		output.WriteString("\n")
	}
	os.Stdout.WriteString(output.String())
	return nil
}
