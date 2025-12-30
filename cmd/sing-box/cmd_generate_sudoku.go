package main

import (
	"os"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/transport/sudoku"

	"github.com/spf13/cobra"
)

func init() {
	commandGenerate.AddCommand(commandGenerateSudokuKeyPair)
}

var commandGenerateSudokuKeyPair = &cobra.Command{
	Use:   "sudoku-keypair",
	Short: "Generate Sudoku key pair",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		err := generateSudokuKeyPair()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func generateSudokuKeyPair() error {
	privateKey, publicKey, err := sudoku.GenKeyPair()
	if err != nil {
		return err
	}
	os.Stdout.WriteString("PrivateKey: " + privateKey + "\n")
	os.Stdout.WriteString("PublicKey: " + publicKey + "\n")
	return nil
}

