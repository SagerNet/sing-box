package main

import (
	"os"
	"slices"
	"strings"
	"time"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var (
	commandAPITailscaleCertificateExportFlagCertificateFile string
	commandAPITailscaleCertificateExportFlagKeyFile         string
	commandAPITailscaleCertificateExportFlagMinValidity     time.Duration
)

var commandAPITailscaleCertificateExport = &cobra.Command{
	Use:   "export <domain>",
	Short: "Export the HTTPS certificate and private key for a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPITailscaleCertificateExport(args[0])
	},
}

func init() {
	commandAPITailscaleCertificateExport.Flags().StringVar(&commandAPITailscaleCertificateExportFlagCertificateFile, "cert-file", "", "Output certificate file (default: <domain>.crt, \"-\" for stdout)")
	commandAPITailscaleCertificateExport.Flags().StringVar(&commandAPITailscaleCertificateExportFlagKeyFile, "key-file", "", "Output private key file (default: <domain>.key, \"-\" for stdout)")
	commandAPITailscaleCertificateExport.Flags().DurationVar(&commandAPITailscaleCertificateExportFlagMinValidity, "min-validity", 0, "Renew the certificate if it expires within this duration")
	commandAPITailscaleCertificate.AddCommand(commandAPITailscaleCertificateExport)
}

func runAPITailscaleCertificateExport(domain string) error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	endpoint, err := fetchTailscaleEndpoint(client)
	if err != nil {
		return err
	}
	certDomains := endpoint.GetCertDomains()
	if len(certDomains) == 0 {
		return E.New("no certificate domains, enable HTTPS in the Tailscale admin console")
	}
	if !slices.Contains(certDomains, domain) {
		return E.New("unknown certificate domain: ", domain, "\nknown domains:\n", formatTailscaleCertificateDomains(certDomains))
	}
	certificateFile := commandAPITailscaleCertificateExportFlagCertificateFile
	if certificateFile == "" {
		certificateFile = domain + ".crt"
	}
	keyFile := commandAPITailscaleCertificateExportFlagKeyFile
	if keyFile == "" {
		keyFile = domain + ".key"
	}
	writeStderrLine("fetching certificate for " + domain)
	certificate, err := client.GetTailscaleCertificate(globalCtx, &daemon.TailscaleCertificateRequest{
		EndpointTag:        endpoint.GetEndpointTag(),
		Domain:             domain,
		MinValiditySeconds: int64(commandAPITailscaleCertificateExportFlagMinValidity / time.Second),
	})
	if err != nil {
		return err
	}
	if certificateFile == "-" && keyFile == "-" {
		_, err = os.Stdout.Write(certificate.GetCertificatePEM())
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(certificate.GetPrivateKeyPEM())
		return err
	}
	err = writeTailscaleCertificateFile(certificateFile, certificate.GetCertificatePEM(), 0o644)
	if err != nil {
		return E.Cause(err, "write certificate")
	}
	err = writeTailscaleCertificateFile(keyFile, certificate.GetPrivateKeyPEM(), 0o600)
	if err != nil {
		return E.Cause(err, "write private key")
	}
	return nil
}

func writeTailscaleCertificateFile(path string, content []byte, mode os.FileMode) error {
	if path == "-" {
		_, err := os.Stdout.Write(content)
		return err
	}
	err := os.WriteFile(path, content, mode)
	if err != nil {
		return err
	}
	writeStderrLine("wrote " + path)
	return nil
}

func formatTailscaleCertificateDomains(domains []string) string {
	return strings.Join(common.Map(domains, func(it string) string {
		return "  " + it
	}), "\n")
}
