package main

import (
	"os"
	"time"

	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/log"

	"github.com/spf13/cobra"
)

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
	commandGenerate.AddCommand(commandGenerateTLSKeyPair)
}

func generateTLSKeyPair(serverName string) error {
	privateKeyPem, publicKeyPem, err := tls.GenerateKeyPair(time.Now, serverName)
	if err != nil {
		return err
	}
	os.Stdout.WriteString(string(privateKeyPem) + "\n")
	os.Stdout.WriteString(string(publicKeyPem) + "\n")
	return nil
}
