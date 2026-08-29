package main

import (
	"github.com/spf13/cobra"
)

var commandAPITailscaleCertificate = &cobra.Command{
	Use:   "certificate",
	Short: "Manage Tailscale HTTPS certificates",
}

func init() {
	commandAPITailscaleCertificate.PersistentFlags().StringVar(&commandAPITailscaleFlagEndpoint, "endpoint", "", commandAPITailscaleEndpointUsage)
	commandAPITailscale.AddCommand(commandAPITailscaleCertificate)
}
