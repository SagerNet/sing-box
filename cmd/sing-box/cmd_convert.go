package main

import (
	"os"
	"path/filepath"

	"github.com/sagernet/sing-box/common/conf"
	"github.com/sagernet/sing-box/common/conf/mergers"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var commandConvertOutput string
var commandConvertOutputFormat string

var commandConvert = &cobra.Command{
	Use:   "convert",
	Short: "Convert configuration",
	Long: `Convert and merge configuration files between different formats.

Note: comments will be lost after conversion.`,
	Run: func(cmd *cobra.Command, args []string) {
		err := convert()
		if err != nil {
			log.Fatal(err)
		}
	},
	Args: cobra.NoArgs,
}

func init() {
	commandConvert.Flags().StringVarP(&commandConvertOutput, "output", "o", "", "output file")
	commandConvert.Flags().StringVarP(&commandConvertOutputFormat, "output-format", "f", string(mergers.FormatJSON), "output format: json, yaml, toml")
	mainCommand.AddCommand(commandConvert)
}

func convert() error {
	var (
		configContent []byte
		err           error
	)
	format := mergers.ParseFormat(configFormat)
	outFormat := mergers.ParseFormat(commandConvertOutputFormat)
	switch outFormat {
	case mergers.FormatJSON,
		mergers.FormatYAML,
		mergers.FormatTOML:
		// ok
	default:
		return E.New("unsupported output format: ", commandConvertOutputFormat)
	}
	if len(configPaths) == 1 && configPaths[0] == "stdin" {
		configContent, err = conf.ReaderToJSON(os.Stdin, format)
	} else {
		configContent, err = conf.FilesToJSON(configPaths, format, configRecursive)
	}
	if err != nil {
		return E.Cause(err, "read config")
	}

	var options option.Options
	err = options.UnmarshalJSON(configContent)
	if err != nil {
		return E.Cause(err, "decode config")
	}
	content, err := conf.Sprint(options, outFormat)
	if err != nil {
		return E.Cause(err, "encode config")
	}

	if commandConvertOutput == "" {
		os.Stdout.WriteString(content + "\n")
		return nil
	}

	output, err := os.Create(commandConvertOutput)
	if err != nil {
		return E.Cause(err, "open output")
	}
	_, err = output.WriteString(content)
	output.Close()
	if err != nil {
		return E.Cause(err, "write output")
	}
	outputPath, _ := filepath.Abs(commandConvertOutput)
	os.Stderr.WriteString(outputPath + "\n")
	return nil
}
