package main

import (
	"io"
	"os"
	"strings"

	"github.com/sagernet/sing-box/common/srs"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"

	"github.com/spf13/cobra"
)

var flagRuleSetCompileOutput string

const flagRuleSetCompileDefaultOutput = "<file_name>.srs"

var commandRuleSetCompile = &cobra.Command{
	Use:   "compile [source-path]",
	Short: "Compile rule-set json to binary",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := compileRuleSet(args[0])
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandRuleSet.AddCommand(commandRuleSetCompile)
	commandRuleSetCompile.Flags().StringVarP(&flagRuleSetCompileOutput, "output", "o", flagRuleSetCompileDefaultOutput, "Output file")
}

func compileRuleSet(sourcePath string) error {
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
	content, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	plainRuleSet, err := json.UnmarshalExtended[option.PlainRuleSetCompat](content)
	if err != nil {
		return err
	}
	var outputPath string
	if flagRuleSetCompileOutput == flagRuleSetCompileDefaultOutput {
		if strings.HasSuffix(sourcePath, ".json") {
			outputPath = sourcePath[:len(sourcePath)-5] + ".srs"
		} else {
			outputPath = sourcePath + ".srs"
		}
	} else {
		outputPath = flagRuleSetCompileOutput
	}
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	err = srs.Write(outputFile, plainRuleSet.Options, plainRuleSet.Version)
	if err != nil {
		outputFile.Close()
		os.Remove(outputPath)
		return err
	}
	outputFile.Close()
	return nil
}
