package main

import (
	"encoding/base64"
	"os"

	"github.com/sagernet/sing-box/log"

	"github.com/spf13/cobra"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func init() {
	commandGenerate.AddCommand(commandGenerateWireGuardKeyPair)
	commandGenerate.AddCommand(commandGenerateRealityKeyPair)
}

var commandGenerateWireGuardKeyPair = &cobra.Command{
	Use:   "wg-keypair",
	Short: "Generate WireGuard key pair",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		err := generateWireGuardKey()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func generateWireGuardKey() error {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return err
	}
	os.Stdout.WriteString("PrivateKey: " + privateKey.String() + "\n")
	os.Stdout.WriteString("PublicKey: " + privateKey.PublicKey().String() + "\n")
	return nil
}

var commandGenerateRealityKeyPair = &cobra.Command{
	Use:   "reality-keypair",
	Short: "Generate reality key pair",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		err := generateRealityKey()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func generateRealityKey() error {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return err
	}
	publicKey := privateKey.PublicKey()
	os.Stdout.WriteString("PrivateKey: " + base64.RawURLEncoding.EncodeToString(privateKey[:]) + "\n")
	os.Stdout.WriteString("PublicKey: " + base64.RawURLEncoding.EncodeToString(publicKey[:]) + "\n")
	return nil
}
