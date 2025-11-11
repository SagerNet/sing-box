package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
	singJson "github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
		optionsEntry.options, err = badjson.Omitempty(globalCtx, optionsEntry.options)
		if err != nil {
			return err
		}

		var formattedContent []byte
		isYAML := isYAMLFile(optionsEntry.path)

		if isYAML {
			// Format as YAML
			var data interface{}
			err = json.Unmarshal(optionsEntry.content, &data)
			if err != nil {
				return E.Cause(err, "unmarshal config for YAML formatting")
			}
			formattedContent, err = yaml.Marshal(data)
			if err != nil {
				return E.Cause(err, "encode config as YAML")
			}
		} else {
			// Format as JSON
			buffer := new(bytes.Buffer)
			encoder := singJson.NewEncoder(buffer)
			encoder.SetIndent("", "  ")
			err = encoder.Encode(optionsEntry.options)
			if err != nil {
				return E.Cause(err, "encode config")
			}
			formattedContent = buffer.Bytes()
		}

		outputPath, _ := filepath.Abs(optionsEntry.path)
		if !commandFormatFlagWrite {
			if len(optionsList) > 1 {
				os.Stdout.WriteString(outputPath + "\n")
			}
			os.Stdout.Write(formattedContent)
			if !isYAML {
				os.Stdout.WriteString("\n")
			}
			continue
		}
		if bytes.Equal(optionsEntry.content, formattedContent) {
			continue
		}
		output, err := os.Create(optionsEntry.path)
		if err != nil {
			return E.Cause(err, "open output")
		}
		_, err = output.Write(formattedContent)
		output.Close()
		if err != nil {
			return E.Cause(err, "write output")
		}
		os.Stderr.WriteString(outputPath + "\n")
	}
	return nil
}
