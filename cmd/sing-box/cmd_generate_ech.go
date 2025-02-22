package main

import (
	"os"

	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/log"

	"github.com/spf13/cobra"
)

var commandGenerateECHKeyPair = &cobra.Command{
	Use:   "ech-keypair <plain_server_name>",
	Short: "Generate TLS ECH key pair",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := generateECHKeyPair(args[0])
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandGenerate.AddCommand(commandGenerateECHKeyPair)
}

func generateECHKeyPair(serverName string) error {
	configPem, keyPem, err := tls.ECHKeygenDefault(serverName)
	if err != nil {
		return err
	}
	os.Stdout.WriteString(configPem)
	os.Stdout.WriteString(keyPem)
	return nil
}
