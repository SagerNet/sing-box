//go:build go1.20

package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"os"

	"github.com/sagernet/sing-box/log"

	"github.com/spf13/cobra"
)

var commandGenerateVAPIDKeyPair = &cobra.Command{
	Use:   "vapid-keypair",
	Short: "Generate VAPID key pair",
	Run: func(cmd *cobra.Command, args []string) {
		err := generateVAPIDKeyPair()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandGenerate.AddCommand(commandGenerateVAPIDKeyPair)
}

func generateVAPIDKeyPair() error {
	privateKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	publicKey := privateKey.PublicKey()
	os.Stdout.WriteString("PrivateKey: " + base64.RawURLEncoding.EncodeToString(privateKey.Bytes()) + "\n")
	os.Stdout.WriteString("PublicKey: " + base64.RawURLEncoding.EncodeToString(publicKey.Bytes()) + "\n")
	return nil
}
