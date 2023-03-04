package generate

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"os"

	"github.com/sagernet/sing-box/log"

	"github.com/spf13/cobra"
)

var CommandGeneratePSK = &cobra.Command{
	Use:   "psk",
	Short: "Generate a random PSK",
	Run: func(cmd *cobra.Command, args []string) {
		if size > 0 {
			encoder := base64.StdEncoding.EncodeToString
			if outputHex {
				encoder = hex.EncodeToString
			}

			psk := make([]byte, size)
			_, err := rand.Read(psk)
			if err != nil {
				log.Fatal(err)
			}

			os.Stdout.WriteString(encoder(psk) + "\n")
		} else {
			cmd.Help()
		}
	},
}

var size int

func init() {
	CommandGeneratePSK.Flags().BoolVarP(&outputHex, "hex", "H", false, "print hex format")
	CommandGeneratePSK.Flags().IntVarP(&size, "size", "s", 0, "PSK size")
}
