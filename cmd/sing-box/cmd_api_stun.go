package main

import (
	"fmt"
	"os"

	"github.com/sagernet/sing-box/common/stun"
	"github.com/sagernet/sing-box/daemon"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var (
	commandAPISTUNFlagServer   string
	commandAPISTUNFlagOutbound string
)

var commandAPISTUN = &cobra.Command{
	Use:   "stun",
	Short: "Run a STUN test",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPISTUN()
	},
}

func init() {
	commandAPISTUN.Flags().StringVar(&commandAPISTUNFlagServer, "server", stun.DefaultServer, "STUN server address")
	commandAPISTUN.Flags().StringVarP(&commandAPISTUNFlagOutbound, "outbound", "o", "", "Use specified tag instead of default outbound")
	commandAPIRoot.AddCommand(commandAPISTUN)
}

func runAPISTUN() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	stream, err := client.StartSTUNTest(globalCtx, &daemon.STUNTestRequest{
		Server:      commandAPISTUNFlagServer,
		OutboundTag: commandAPISTUNFlagOutbound,
	})
	if err != nil {
		return err
	}
	writeStderrLine("==== STUN TEST ====")
	for {
		progress, recvErr := stream.Recv()
		if recvErr != nil {
			return recvErr
		}
		if !progress.GetIsFinal() {
			switch stun.Phase(progress.GetPhase()) {
			case stun.PhaseBinding:
				if progress.GetExternalAddr() != "" {
					writeProgress(fmt.Sprintf("External Address: %s (%d ms)", progress.GetExternalAddr(), progress.GetLatencyMs()))
				} else {
					writeProgress("Sending binding request...")
				}
			case stun.PhaseNATMapping:
				writeProgress("Detecting NAT mapping behavior...")
			case stun.PhaseNATFiltering:
				writeProgress("Detecting NAT filtering behavior...")
			}
			continue
		}
		writeStderrLine("")
		if progress.GetError() != "" {
			return E.New(progress.GetError())
		}
		fmt.Fprintf(os.Stdout, "External Address: %s\n", progress.GetExternalAddr())
		fmt.Fprintf(os.Stdout, "Latency:          %d ms\n", progress.GetLatencyMs())
		if progress.GetNatTypeSupported() {
			fmt.Fprintf(os.Stdout, "NAT Mapping:      %s\n", stun.NATMapping(progress.GetNatMapping()))
			fmt.Fprintf(os.Stdout, "NAT Filtering:    %s\n", stun.NATFiltering(progress.GetNatFiltering()))
		} else {
			fmt.Fprintln(os.Stdout, "NAT Type Detection: not supported by server")
		}
		return nil
	}
}
