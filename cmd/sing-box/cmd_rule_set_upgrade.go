package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"

	"github.com/spf13/cobra"
)

var commandRuleSetUpgradeFlagWrite bool

var commandRuleSetUpgrade = &cobra.Command{
	Use:   "upgrade <source-path>",
	Short: "Upgrade rule-set json",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := upgradeRuleSet(args[0])
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandRuleSetUpgrade.Flags().BoolVarP(&commandRuleSetUpgradeFlagWrite, "write", "w", false, "write result to (source) file instead of stdout")
	commandRuleSet.AddCommand(commandRuleSetUpgrade)
}

func upgradeRuleSet(sourcePath string) error {
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
	plainRuleSetCompat, err := json.UnmarshalExtended[option.PlainRuleSetCompat](content)
	if err != nil {
		return err
	}
	switch plainRuleSetCompat.Version {
	case C.RuleSetVersion1:
	default:
		log.Info("already up-to-date")
		return nil
	}
	plainRuleSetCompat.Options, err = plainRuleSetCompat.Upgrade()
	if err != nil {
		return err
	}
	plainRuleSetCompat.Version = C.RuleSetVersionCurrent
	buffer := new(bytes.Buffer)
	encoder := json.NewEncoder(buffer)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(plainRuleSetCompat)
	if err != nil {
		return E.Cause(err, "encode config")
	}
	outputPath, _ := filepath.Abs(sourcePath)
	if !commandRuleSetUpgradeFlagWrite || sourcePath == "stdin" {
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
