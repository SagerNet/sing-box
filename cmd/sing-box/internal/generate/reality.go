package generate

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"os"

	"github.com/sagernet/sing-box/log"
	F "github.com/sagernet/sing/common/format"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/curve25519"
)

var CommandGenerateReality = &cobra.Command{
	Use:   "reality-key",
	Short: "Generate a REALITY protocol X25519 key pair",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		encoder := base64.RawURLEncoding.EncodeToString
		if outputHex {
			encoder = hex.EncodeToString
		}

		var privateKey [curve25519.ScalarSize]byte
		if input == "" {
			_, err := rand.Read(privateKey[:])
			if err != nil {
				log.Fatal(err)
			}
		} else {
			src := []byte(input)
			n, _ := base64.RawURLEncoding.Decode(privateKey[:], src)
			if n != curve25519.ScalarSize {
				n, _ = hex.Decode(privateKey[:], src)
				if n != curve25519.ScalarSize {
					log.Fatal("invalid input private key")
				}
			}
		}

		publicKey, err := curve25519.X25519(privateKey[:], curve25519.Basepoint)
		if err != nil {
			log.Fatal(err)
		}

		os.Stdout.WriteString(F.ToString(
			"Private key: ", encoder(privateKey[:]),
			"\nPublic key: ", encoder(publicKey), "\n"))
	},
}

func init() {
	CommandGenerateReality.Flags().BoolVarP(&outputHex, "hex", "H", false, "print hex format")
	CommandGenerateReality.Flags().StringVarP(&input, "input", "i", "", "generate from specified private key")
}
