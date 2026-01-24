package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/route/rule"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/spf13/cobra"
)

var flagRuleSetMatchFormat string

var commandRuleSetMatch = &cobra.Command{
	Use:   "match <rule-set path> <IP address/domain>",
	Short: "Check if an IP address or a domain matches the rule-set",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		err := ruleSetMatch(args[0], args[1])
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandRuleSetMatch.Flags().StringVarP(&flagRuleSetMatchFormat, "format", "f", "source", "rule-set format")
	commandRuleSet.AddCommand(commandRuleSetMatch)
}

func ruleSetMatch(sourcePath string, domain string) error {
	var (
		reader io.Reader
		err    error
	)
	if sourcePath == "stdin" {
		reader = os.Stdin
	} else {
		reader, err = os.Open(sourcePath)
		if err != nil {
			return E.Cause(err, "read rule-set")
		}
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		return E.Cause(err, "read rule-set")
	}
	if flagRuleSetMatchFormat == "" {
		switch filepath.Ext(sourcePath) {
		case ".json":
			flagRuleSetMatchFormat = C.RuleSetFormatSource
		case ".srs":
			flagRuleSetMatchFormat = C.RuleSetFormatBinary
		}
	}
	var ruleSet option.PlainRuleSetCompat
	switch flagRuleSetMatchFormat {
	case C.RuleSetFormatSource:
		ruleSet, err = json.UnmarshalExtended[option.PlainRuleSetCompat](content)
		if err != nil {
			return err
		}
	case C.RuleSetFormatBinary:
		ruleSet, err = srs.Read(bytes.NewReader(content), false)
		if err != nil {
			return err
		}
	default:
		return E.New("unknown rule-set format: ", flagRuleSetMatchFormat)
	}
	plainRuleSet, err := ruleSet.Upgrade()
	if err != nil {
		return err
	}
	ipAddress := M.ParseAddr(domain)
	var metadata adapter.InboundContext
	if ipAddress.IsValid() {
		metadata.Destination = M.SocksaddrFrom(ipAddress, 0)
	} else {
		metadata.Domain = domain
	}
	for i, ruleOptions := range plainRuleSet.Rules {
		var currentRule adapter.HeadlessRule
		currentRule, err = rule.NewHeadlessRule(context.Background(), ruleOptions)
		if err != nil {
			return E.Cause(err, "parse rule_set.rules.[", i, "]")
		}
		if currentRule.Match(&metadata) {
			println(F.ToString("match rules.[", i, "]: ", currentRule))
		}
	}
	return nil
}
