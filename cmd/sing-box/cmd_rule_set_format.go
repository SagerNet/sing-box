package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/sagernet/sing-box/common/json"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var commandRuleSetFormatFlagWrite bool

var commandRuleSetFormat = &cobra.Command{
	Use:   "format <source-path>",
	Short: "Format rule-set json",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := formatRuleSet(args[0])
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandRuleSetFormat.Flags().BoolVarP(&commandRuleSetFormatFlagWrite, "write", "w", false, "write result to (source) file instead of stdout")
	commandRuleSet.AddCommand(commandRuleSetFormat)
}

func formatRuleSet(sourcePath string) error {
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
	decoder := json.NewDecoder(json.NewCommentFilter(bytes.NewReader(content)))
	decoder.DisallowUnknownFields()
	var plainRuleSet option.PlainRuleSetCompat
	err = decoder.Decode(&plainRuleSet)
	if err != nil {
		return err
	}
	ruleSet := plainRuleSet.Upgrade()
	buffer := new(bytes.Buffer)
	encoder := json.NewEncoder(buffer)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(ruleSet)
	if err != nil {
		return E.Cause(err, "encode config")
	}
	outputPath, _ := filepath.Abs(sourcePath)
	if !commandRuleSetFormatFlagWrite || sourcePath == "stdin" {
		os.Stdout.WriteString(buffer.String() + "\n")
		return nil
	}
	if bytes.Equal(content, buffer.Bytes()) {
		return nil
	}
	output, err := os.Create(sourcePath)
	if err != nil {
		return E.Cause(err, "open output")
	}
	_, err = output.Write(buffer.Bytes())
	output.Close()
	if err != nil {
		return E.Cause(err, "write output")
	}
	os.Stderr.WriteString(outputPath + "\n")
	return nil
}
