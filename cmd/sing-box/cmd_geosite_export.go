package main

import (
	"io"
	"os"

	"github.com/sagernet/sing-box/common/geosite"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"

	"github.com/spf13/cobra"
)

var commandGeositeExportOutput string

const commandGeositeExportDefaultOutput = "geosite-<category>.json"

var commandGeositeExport = &cobra.Command{
	Use:   "export <category>",
	Short: "Export geosite category as rule-set",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := geositeExport(args[0])
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandGeositeExport.Flags().StringVarP(&commandGeositeExportOutput, "output", "o", commandGeositeExportDefaultOutput, "Output path")
	commandGeoSite.AddCommand(commandGeositeExport)
}

func geositeExport(category string) error {
	sourceSet, err := geositeReader.Read(category)
	if err != nil {
		return err
	}
	var (
		outputFile   *os.File
		outputWriter io.Writer
	)
	if commandGeositeExportOutput == "stdout" {
		outputWriter = os.Stdout
	} else if commandGeositeExportOutput == commandGeositeExportDefaultOutput {
		outputFile, err = os.Create("geosite-" + category + ".json")
		if err != nil {
			return err
		}
		defer outputFile.Close()
		outputWriter = outputFile
	} else {
		outputFile, err = os.Create(commandGeositeExportOutput)
		if err != nil {
			return err
		}
		defer outputFile.Close()
		outputWriter = outputFile
	}

	encoder := json.NewEncoder(outputWriter)
	encoder.SetIndent("", "  ")
	var headlessRule option.DefaultHeadlessRule
	defaultRule := geosite.Compile(sourceSet)
	headlessRule.Domain = defaultRule.Domain
	headlessRule.DomainSuffix = defaultRule.DomainSuffix
	headlessRule.DomainKeyword = defaultRule.DomainKeyword
	headlessRule.DomainRegex = defaultRule.DomainRegex
	var plainRuleSet option.PlainRuleSetCompat
	plainRuleSet.Version = C.RuleSetVersion2
	plainRuleSet.Options.Rules = []option.HeadlessRule{
		{
			Type:           C.RuleTypeDefault,
			DefaultOptions: headlessRule,
		},
	}
	return encoder.Encode(plainRuleSet)
}
