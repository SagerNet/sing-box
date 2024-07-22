package main

import (
	"io"
	"os"
	"strings"

	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"

	"github.com/spf13/cobra"
)

var flagRuleSetDecompileOutput string

const flagRuleSetDecompileDefaultOutput = "<file_name>.json"

var commandRuleSetDecompile = &cobra.Command{
	Use:   "decompile [binary-path]",
	Short: "Decompile rule-set binary to json",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := decompileRuleSet(args[0])
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandRuleSet.AddCommand(commandRuleSetDecompile)
	commandRuleSetDecompile.Flags().StringVarP(&flagRuleSetDecompileOutput, "output", "o", flagRuleSetDecompileDefaultOutput, "Output file")
}

func decompileRuleSet(sourcePath string) error {
	var (
		reader io.Reader
		err    error
	)
	if sourcePath == "stdin" {
		reader = os.Stdin
	} else {
		reader, err = os.Open(sourcePath)
		if err != nil {
			return err
		}
	}
	plainRuleSet, err := srs.Read(reader, true)
	if err != nil {
		return err
	}
	ruleSet := option.PlainRuleSetCompat{
		Version: C.RuleSetVersion1,
		Options: plainRuleSet,
	}
	var outputPath string
	if flagRuleSetDecompileOutput == flagRuleSetDecompileDefaultOutput {
		if strings.HasSuffix(sourcePath, ".srs") {
			outputPath = sourcePath[:len(sourcePath)-4] + ".json"
		} else {
			outputPath = sourcePath + ".json"
		}
	} else {
		outputPath = flagRuleSetDecompileOutput
	}
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(outputFile)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(ruleSet)
	if err != nil {
		outputFile.Close()
		os.Remove(outputPath)
		return err
	}
	outputFile.Close()
	return nil
}
