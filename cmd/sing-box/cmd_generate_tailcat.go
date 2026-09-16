package main

import (
	"crypto/rand"
	"os"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/protocol/tailscale"

	"github.com/spf13/cobra"
)

func init() {
	commandGenerate.AddCommand(commandGenerateTailcatKeyPair)
}

var commandGenerateTailcatKeyPair = &cobra.Command{
	Use:   "tailcat-keypair",
	Short: "Generate Tailcat key pair",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		err := generateTailcatKey()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func generateTailcatKey() error {
	privateKey := tailscale.NewTailcatPrivateKey()
	var presharedKey [32]byte
	_, err := rand.Read(presharedKey[:])
	if err != nil {
		return err
	}
	os.Stdout.WriteString("PrivateKey: " + tailscale.EncodeTailcatKey(privateKey) + "\n")
	os.Stdout.WriteString("PublicKey: " + tailscale.EncodeTailcatKey(tailscale.TailcatPublicKey(privateKey)) + "\n")
	os.Stdout.WriteString("DiscoKey: " + tailscale.EncodeTailcatKey(tailscale.TailcatPublicKey(tailscale.TailcatDiscoPrivateKey(privateKey))) + "\n")
	os.Stdout.WriteString("PreSharedKey: " + tailscale.EncodeTailcatKey(presharedKey) + "\n")
	os.Stdout.WriteString("Address: " + tailscale.TailcatAddress(tailscale.TailcatPublicKey(privateKey)).String() + "\n")
	return nil
}
