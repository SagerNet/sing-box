package main

import (
	"crypto/rand"
	"os"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/transport/mtproto"

	"github.com/spf13/cobra"
)

var mainCommand = &cobra.Command{
	Use:  "generate-mtproto-secret <hostname>",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := generate(args[0])
		if err != nil {
			log.Fatal(err)
		}
	},
}

func main() {
	if err := mainCommand.Execute(); err != nil {
		log.Fatal(err)
	}
}

func generate(hostname string) error {
	secret := mtproto.Secret{
		Host: hostname,
	}
	_, err := rand.Read(secret.Key[:])
	if err != nil {
		return err
	}
	_, err = os.Stdout.WriteString(secret.String() + "\n")
	return err
}
