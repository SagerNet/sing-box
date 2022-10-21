package main

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/sagernet/sing-box/common/conf"
	"github.com/sagernet/sing-box/common/json"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var commandFormatFlagWrite bool

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
	commandFormat.Flags().BoolVarP(&commandFormatFlagWrite, "write", "w", false, "write result to (source) file instead of stdout")
	mainCommand.AddCommand(commandFormat)
}

func format() error {
	files, err := conf.ResolveFiles(configPaths, configRecursive)
	if err != nil {
		return E.Cause(err, "resolve config files")
	}
	if len(files) == 0 {
		return E.New("no config file found")
	}
	// use conf.Merge even if there's only one config file, make
	// it has the same behavior between one and multiple files.
	configContent, err := conf.Merge(files)
	if err != nil {
		return E.Cause(err, "read config")
	}
	var options option.Options
	err = options.UnmarshalJSON(configContent)
	if err != nil {
		return E.Cause(err, "decode config")
	}
	buffer := new(bytes.Buffer)
	encoder := json.NewEncoder(buffer)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(options)
	if err != nil {
		return E.Cause(err, "encode config")
	}
	flagIgnored := false
	if commandFormatFlagWrite && len(files) > 1 {
		commandFormatFlagWrite = false
		flagIgnored = true
	}
	if !commandFormatFlagWrite {
		os.Stdout.WriteString(buffer.String() + "\n")
		if flagIgnored {
			log.Warn("--write flag is ignored due to more than one configuration file specified")
		}
		return nil
	}
	if bytes.Equal(configContent, buffer.Bytes()) {
		return nil
	}
	configPath := files[0]
	output, err := os.Create(configPath)
	if err != nil {
		return E.Cause(err, "open output")
	}
	_, err = output.Write(buffer.Bytes())
	output.Close()
	if err != nil {
		return E.Cause(err, "write output")
	}
	outputPath, _ := filepath.Abs(configPath)
	os.Stderr.WriteString(outputPath + "\n")
	return nil
}
