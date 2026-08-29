package main

import (
	"github.com/spf13/cobra"
)

var commandAPITailscaleCertificateList = &cobra.Command{
	Use:   "list",
	Short: "List domains that certificates can be issued for",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPITailscaleCertificateList()
	},
}

func init() {
	commandAPITailscaleCertificate.AddCommand(commandAPITailscaleCertificateList)
}

func runAPITailscaleCertificateList() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	endpoint, err := fetchTailscaleEndpoint(client)
	if err != nil {
		return err
	}
	table := tableWriter{
		header:       []string{"DOMAIN"},
		emptyMessage: "no certificate domains, enable HTTPS in the Tailscale admin console",
	}
	for _, domain := range endpoint.GetCertDomains() {
		table.addRow(domain)
	}
	table.flush()
	return nil
}
