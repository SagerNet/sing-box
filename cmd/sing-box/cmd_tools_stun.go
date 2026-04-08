package main

import (
	"fmt"
	"os"

	"github.com/sagernet/sing-box/common/stun"
	"github.com/sagernet/sing-box/log"

	"github.com/spf13/cobra"
)

var commandSTUNFlagServer string

var commandSTUN = &cobra.Command{
	Use:   "stun",
	Short: "Run a STUN test",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		err := runSTUN()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandSTUN.Flags().StringVarP(&commandSTUNFlagServer, "server", "s", stun.DefaultServer, "STUN server address")
	commandTools.AddCommand(commandSTUN)
}

func runSTUN() error {
	instance, err := createPreStartedClient()
	if err != nil {
		return err
	}
	defer instance.Close()

	dialer, err := createDialer(instance, commandToolsFlagOutbound)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "==== STUN TEST ====")

	result, err := stun.Run(stun.Options{
		Server:  commandSTUNFlagServer,
		Dialer:  dialer,
		Context: globalCtx,
		OnProgress: func(p stun.Progress) {
			switch p.Phase {
			case stun.PhaseBinding:
				if p.ExternalAddr != "" {
					fmt.Fprintf(os.Stderr, "\rExternal Address: %s (%d ms)", p.ExternalAddr, p.LatencyMs)
				} else {
					fmt.Fprint(os.Stderr, "\rSending binding request...")
				}
			case stun.PhaseNATMapping:
				fmt.Fprint(os.Stderr, "\rDetecting NAT mapping behavior...")
			case stun.PhaseNATFiltering:
				fmt.Fprint(os.Stderr, "\rDetecting NAT filtering behavior...")
			}
		},
	})
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "External Address: %s\n", result.ExternalAddr)
	fmt.Fprintf(os.Stderr, "Latency:          %d ms\n", result.LatencyMs)
	if result.NATTypeSupported {
		fmt.Fprintf(os.Stderr, "NAT Mapping:      %s\n", result.NATMapping)
		fmt.Fprintf(os.Stderr, "NAT Filtering:    %s\n", result.NATFiltering)
	} else {
		fmt.Fprintln(os.Stderr, "NAT Type Detection: not supported by server")
	}
	return nil
}
