package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"os"
	"strconv"

	"github.com/sagernet/sing-box/log"

	"github.com/gofrs/uuid/v5"
	"github.com/spf13/cobra"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var commandGenerate = &cobra.Command{
	Use:   "generate",
	Short: "Generate things",
}

func init() {
	commandGenerate.AddCommand(commandGenerateUUID)
	commandGenerate.AddCommand(commandGenerateRandom)
	commandGenerate.AddCommand(commandGenerateWireGuardKeyPair)
	commandGenerate.AddCommand(commandGenerateRealityKeyPair)
	mainCommand.AddCommand(commandGenerate)
}

var (
	outputBase64 bool
	outputHex    bool
)

var commandGenerateRandom = &cobra.Command{
	Use:   "rand <length>",
	Short: "Generate random bytes",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := generateRandom(args)
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandGenerateRandom.Flags().BoolVar(&outputBase64, "base64", false, "Generate base64 string")
	commandGenerateRandom.Flags().BoolVar(&outputHex, "hex", false, "Generate hex string")
}

func generateRandom(args []string) error {
	length, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}

	randomBytes := make([]byte, length)
	_, err = rand.Read(randomBytes)
	if err != nil {
		return err
	}

	if outputBase64 {
		_, err = os.Stdout.WriteString(base64.StdEncoding.EncodeToString(randomBytes) + "\n")
	} else if outputHex {
		_, err = os.Stdout.WriteString(hex.EncodeToString(randomBytes) + "\n")
	} else {
		_, err = os.Stdout.Write(randomBytes)
	}

	return err
}

var commandGenerateUUID = &cobra.Command{
	Use:   "uuid",
	Short: "Generate UUID string",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		err := generateUUID()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func generateUUID() error {
	newUUID, err := uuid.NewV4()
	if err != nil {
		return err
	}
	_, err = os.Stdout.WriteString(newUUID.String() + "\n")
	return err
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
