package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/rw"

	"github.com/spf13/cobra"
)

var (
	ruleSetPaths       []string
	ruleSetDirectories []string
)

var commandRuleSetMerge = &cobra.Command{
	Use:   "merge <output-path>",
	Short: "Merge rule-set source files",
	Run: func(cmd *cobra.Command, args []string) {
		err := mergeRuleSet(args[0])
		if err != nil {
			log.Fatal(err)
		}
	},
	Args: cobra.ExactArgs(1),
}

func init() {
	commandRuleSetMerge.Flags().StringArrayVarP(&ruleSetPaths, "config", "c", nil, "set input rule-set file path")
	commandRuleSetMerge.Flags().StringArrayVarP(&ruleSetDirectories, "config-directory", "C", nil, "set input rule-set directory path")
	commandRuleSet.AddCommand(commandRuleSetMerge)
}

type RuleSetEntry struct {
	content []byte
	path    string
	options option.PlainRuleSetCompat
}

func readRuleSetAt(path string) (*RuleSetEntry, error) {
	var (
		configContent []byte
		err           error
	)
	if path == "stdin" {
		configContent, err = io.ReadAll(os.Stdin)
	} else {
		configContent, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, E.Cause(err, "read config at ", path)
	}
	options, err := json.UnmarshalExtendedContext[option.PlainRuleSetCompat](globalCtx, configContent)
	if err != nil {
		return nil, E.Cause(err, "decode config at ", path)
	}
	return &RuleSetEntry{
		content: configContent,
		path:    path,
		options: options,
	}, nil
}

func readRuleSet() ([]*RuleSetEntry, error) {
	var optionsList []*RuleSetEntry
	for _, path := range ruleSetPaths {
		optionsEntry, err := readRuleSetAt(path)
		if err != nil {
			return nil, err
		}
		optionsList = append(optionsList, optionsEntry)
	}
	for _, directory := range ruleSetDirectories {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return nil, E.Cause(err, "read rule-set directory at ", directory)
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".json") || entry.IsDir() {
				continue
			}
			optionsEntry, err := readRuleSetAt(filepath.Join(directory, entry.Name()))
			if err != nil {
				return nil, err
			}
			optionsList = append(optionsList, optionsEntry)
		}
	}
	sort.Slice(optionsList, func(i, j int) bool {
		return optionsList[i].path < optionsList[j].path
	})
	return optionsList, nil
}

func readRuleSetAndMerge() (option.PlainRuleSetCompat, error) {
	optionsList, err := readRuleSet()
	if err != nil {
		return option.PlainRuleSetCompat{}, err
	}
	if len(optionsList) == 1 {
		return optionsList[0].options, nil
	}
	var optionVersion uint8
	for _, options := range optionsList {
		if optionVersion < options.options.Version {
			optionVersion = options.options.Version
		}
	}
	var mergedMessage json.RawMessage
	for _, options := range optionsList {
		mergedMessage, err = badjson.MergeJSON(globalCtx, options.options.RawMessage, mergedMessage, false)
		if err != nil {
			return option.PlainRuleSetCompat{}, E.Cause(err, "merge config at ", options.path)
		}
	}
	mergedOptions, err := json.UnmarshalExtendedContext[option.PlainRuleSetCompat](globalCtx, mergedMessage)
	if err != nil {
		return option.PlainRuleSetCompat{}, E.Cause(err, "unmarshal merged config")
	}
	mergedOptions.Version = optionVersion
	return mergedOptions, nil
}

func mergeRuleSet(outputPath string) error {
	mergedOptions, err := readRuleSetAndMerge()
	if err != nil {
		return err
	}
	buffer := new(bytes.Buffer)
	encoder := json.NewEncoder(buffer)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(mergedOptions)
	if err != nil {
		return E.Cause(err, "encode config")
	}
	if existsContent, err := os.ReadFile(outputPath); err != nil {
		if string(existsContent) == buffer.String() {
			return nil
		}
	}
	err = rw.MkdirParent(outputPath)
	if err != nil {
		return err
	}
	err = os.WriteFile(outputPath, buffer.Bytes(), 0o644)
	if err != nil {
		return err
	}
	outputPath, _ = filepath.Abs(outputPath)
	os.Stderr.WriteString(outputPath + "\n")
	return nil
}
