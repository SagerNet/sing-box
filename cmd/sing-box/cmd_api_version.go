package main

import (
	"os"

	"github.com/sagernet/sing-box/daemon"
	F "github.com/sagernet/sing/common/format"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
)

var commandAPIVersion = &cobra.Command{
	Use:   "version",
	Short: "Print the API service version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIVersion()
	},
}

func init() {
	commandAPIRoot.AddCommand(commandAPIVersion)
}

func runAPIVersion() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	version, err := client.GetVersion(globalCtx, &emptypb.Empty{})
	if err != nil {
		return err
	}
	if version.GetApiVersion() != daemon.APIVersion {
		writeStderrLine(F.ToString("warning: server API version ", version.GetApiVersion(), ", client ", daemon.APIVersion))
	}
	os.Stdout.WriteString(version.GetVersion() + "\n")
	return nil
}
