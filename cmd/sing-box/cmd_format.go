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

var commandFormatWrite string
var commandEncodeFormat string

var commandFormat = &cobra.Command{
	Use:   "format",
	Short: "Format configuration",
	Run: func(cmd *cobra.Command, args []string) {
		err := format()
		if err != nil {
			log.Fatal(err)
		}
	},
	Args: cobra.NoArgs,
}

func init() {
	commandFormat.Flags().StringVarP(&commandFormatWrite, "write", "w", "", "write result to (source) file instead of stdout")
	commandFormat.Flags().StringVarP(&commandEncodeFormat, "encode", "e", string(mergers.FormatJSON), "encode format")
	mainCommand.AddCommand(commandFormat)
}

func format() error {
	var (
		configContent []byte
		err           error
	)
	format := mergers.ParseFormat(configFormat)
	encode := mergers.ParseFormat(commandEncodeFormat)
	if encode == mergers.FormatAuto {
		encode = mergers.FormatJSON
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
	content, err := conf.Sprint(options, encode)
	if err != nil {
		return E.Cause(err, "encode config")
	}

	if commandFormatWrite == "" {
		os.Stdout.WriteString(content + "\n")
		return nil
	}

	output, err := os.Create(commandFormatWrite)
	if err != nil {
		return E.Cause(err, "open output")
	}
	_, err = output.WriteString(content)
	output.Close()
	if err != nil {
		return E.Cause(err, "write output")
	}
	outputPath, _ := filepath.Abs(commandFormatWrite)
	os.Stderr.WriteString(outputPath + "\n")
	return nil
}
