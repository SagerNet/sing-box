package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/spf13/cobra"
)

var commandRuleSetMergeFlagWriteDst string

var commandRuleSetMerge = &cobra.Command{
	Use:   "merge [flags] <source-path-1> <source-path-2> [<source-path-3> ...]",
	Short: "Merge multiple rule-set json files",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return errors.New("requires at least two input files")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		err := mergeRuleSets(args)
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandRuleSetMerge.Flags().StringVarP(&commandRuleSetMergeFlagWriteDst, "write", "w", "", "write result to a file instead of stdout")
	commandRuleSet.AddCommand(commandRuleSetMerge)
}

func mergeRuleSets(paths []string) error {
	var ver uint8
	var merged []option.HeadlessRule
	for i, p := range paths {
		var (
			reader io.Reader
			err    error
		)
		if p == "stdin" {
			reader = os.Stdin
		} else {
			reader, err = os.Open(p)
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
		if i == 0 {
			ver = plainRuleSet.Version
		} else {
			if plainRuleSet.Version != ver {
				return fmt.Errorf("version mismatch, use `sing-box rule-set upgrade` to upgrade first")
			}
		}
		merged = append(merged, plainRuleSet.Options.Rules...)
	}
	buffer := new(bytes.Buffer)
	encoder := json.NewEncoder(buffer)
	encoder.SetIndent("", "  ")
	var err = encoder.Encode(merged)
	if err != nil {
		return E.Cause(err, "encode merged result")
	}
	if commandRuleSetMergeFlagWriteDst == "" {
		_, err := os.Stdout.WriteString(buffer.String() + "\n")
		if err != nil {
			return err
		}
		return nil
	}
	output, err := os.Create(commandRuleSetMergeFlagWriteDst)
	if err != nil {
		return E.Cause(err, "open output")
	}
	_, err = output.Write(buffer.Bytes())
	if err != nil {
		return E.Cause(err, "write output")
	}
	err = output.Close()
	if err != nil {
		return E.Cause(err, "close output")
	}
	return nil
}
