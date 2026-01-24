package main

import (
	"io"
	"os"
	"strings"

	"github.com/sagernet/sing-box/common/convertor/adguard"
	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var (
	flagRuleSetConvertType   string
	flagRuleSetConvertOutput string
)

var commandRuleSetConvert = &cobra.Command{
	Use:   "convert [source-path]",
	Short: "Convert adguard DNS filter to rule-set",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := convertRuleSet(args[0])
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandRuleSet.AddCommand(commandRuleSetConvert)
	commandRuleSetConvert.Flags().StringVarP(&flagRuleSetConvertType, "type", "t", "", "Source type, available: adguard")
	commandRuleSetConvert.Flags().StringVarP(&flagRuleSetConvertOutput, "output", "o", flagRuleSetCompileDefaultOutput, "Output file")
}

func convertRuleSet(sourcePath string) error {
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
	var rules []option.HeadlessRule
	switch flagRuleSetConvertType {
	case "adguard":
		rules, err = adguard.ToOptions(reader, log.StdLogger())
	case "":
		return E.New("source type is required")
	default:
		return E.New("unsupported source type: ", flagRuleSetConvertType)
	}
	if err != nil {
		return err
	}
	var outputPath string
	if flagRuleSetConvertOutput == flagRuleSetCompileDefaultOutput {
		if strings.HasSuffix(sourcePath, ".txt") {
			outputPath = sourcePath[:len(sourcePath)-4] + ".srs"
		} else {
			outputPath = sourcePath + ".srs"
		}
	} else {
		outputPath = flagRuleSetConvertOutput
	}
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outputFile.Close()
	err = srs.Write(outputFile, option.PlainRuleSet{Rules: rules}, C.RuleSetVersion2)
	if err != nil {
		outputFile.Close()
		os.Remove(outputPath)
		return err
	}
	outputFile.Close()
	return nil
}
