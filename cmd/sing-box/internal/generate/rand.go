package generate

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"os"
	"strconv"

	"github.com/sagernet/sing-box/log"

	"github.com/spf13/cobra"
)

var CommandGenerateRandom = &cobra.Command{
	Use:   "rand",
	Short: "Generate random bytes",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		length, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatal(err)
		}

		encoder := base64.StdEncoding.EncodeToString
		if outputHex {
			encoder = hex.EncodeToString
		}

		bs := make([]byte, length)
		_, err = rand.Read(bs)
		if err != nil {
			log.Fatal(err)
		}

		os.Stdout.WriteString(encoder(bs) + "\n")
	},
}

func init() {
	CommandGenerateRandom.Flags().BoolVarP(&outputHex, "hex", "H", false, "print hex format")
}
