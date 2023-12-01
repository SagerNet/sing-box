package main

import (
	"os"
	"sort"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var commandGeositeLookup = &cobra.Command{
	Use:   "lookup [category] <domain>",
	Short: "Check if a domain is in the geosite",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		var (
			source string
			target string
		)
		switch len(args) {
		case 1:
			target = args[0]
		case 2:
			source = args[0]
			target = args[1]
		}
		err := geositeLookup(source, target)
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandGeoSite.AddCommand(commandGeositeLookup)
}

func geositeLookup(source string, target string) error {
	var sourceMatcherList []struct {
		code    string
		matcher *searchGeositeMatcher
	}
	if source != "" {
		sourceSet, err := geositeReader.Read(source)
		if err != nil {
			return err
		}
		sourceMatcher, err := newSearchGeositeMatcher(sourceSet)
		if err != nil {
			return E.Cause(err, "compile code: "+source)
		}
		sourceMatcherList = []struct {
			code    string
			matcher *searchGeositeMatcher
		}{
			{
				code:    source,
				matcher: sourceMatcher,
			},
		}

	} else {
		for _, code := range geositeCodeList {
			sourceSet, err := geositeReader.Read(code)
			if err != nil {
				return err
			}
			sourceMatcher, err := newSearchGeositeMatcher(sourceSet)
			if err != nil {
				return E.Cause(err, "compile code: "+code)
			}
			sourceMatcherList = append(sourceMatcherList, struct {
				code    string
				matcher *searchGeositeMatcher
			}{
				code:    code,
				matcher: sourceMatcher,
			})
		}
	}
	sort.SliceStable(sourceMatcherList, func(i, j int) bool {
		return sourceMatcherList[i].code < sourceMatcherList[j].code
	})

	for _, matcherItem := range sourceMatcherList {
		if matchRule := matcherItem.matcher.Match(target); matchRule != "" {
			os.Stdout.WriteString("Match code (")
			os.Stdout.WriteString(matcherItem.code)
			os.Stdout.WriteString(") ")
			os.Stdout.WriteString(matchRule)
			os.Stdout.WriteString("\n")
		}
	}
	return nil
}
