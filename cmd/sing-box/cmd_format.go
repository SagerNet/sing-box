package main

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/sagernet/sing-box/common/json"
	"github.com/sagernet/sing-box/log"
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
	optionsList, err := readConfig()
	if err != nil {
		return err
	}
	for _, optionsEntry := range optionsList {
		buffer := new(bytes.Buffer)
		encoder := json.NewEncoder(buffer)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(optionsEntry.options)
		if err != nil {
			return E.Cause(err, "encode config")
		}
		outputPath, _ := filepath.Abs(optionsEntry.path)
		if !commandFormatFlagWrite {
			if len(optionsList) > 1 {
				os.Stdout.WriteString(outputPath + "\n")
			}
			os.Stdout.WriteString(buffer.String() + "\n")
			continue
		}
		if bytes.Equal(optionsEntry.content, buffer.Bytes()) {
			continue
		}
		output, err := os.Create(optionsEntry.path)
		if err != nil {
			return E.Cause(err, "open output")
		}
		_, err = output.Write(buffer.Bytes())
		output.Close()
		if err != nil {
			return E.Cause(err, "write output")
		}
		os.Stderr.WriteString(outputPath + "\n")
	}
	return nil
}
