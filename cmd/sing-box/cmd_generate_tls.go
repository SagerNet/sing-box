package main

import (
	"os"
	"time"

	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/log"

	"github.com/spf13/cobra"
)

var flagGenerateTLSKeyPairMonths int

var commandGenerateTLSKeyPair = &cobra.Command{
	Use:   "tls-keypair <server_name>",
	Short: "Generate TLS self sign key pair",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := generateTLSKeyPair(args[0])
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandGenerateTLSKeyPair.Flags().IntVarP(&flagGenerateTLSKeyPairMonths, "months", "m", 1, "Valid months")
	commandGenerate.AddCommand(commandGenerateTLSKeyPair)
}

func generateTLSKeyPair(serverName string) error {
	privateKeyPem, publicKeyPem, err := tls.GenerateCertificate(nil, nil, time.Now, serverName, time.Now().AddDate(0, flagGenerateTLSKeyPairMonths, 0))
	if err != nil {
		return err
	}
	os.Stdout.WriteString(string(privateKeyPem) + "\n")
	os.Stdout.WriteString(string(publicKeyPem) + "\n")
	return nil
}
