package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/rw"

	"github.com/spf13/cobra"
)

var commandMerge = &cobra.Command{
	Use:   "merge <output-path>",
	Short: "Merge configurations",
	Run: func(cmd *cobra.Command, args []string) {
		err := merge(args[0])
		if err != nil {
			log.Fatal(err)
		}
	},
	Args: cobra.ExactArgs(1),
}

func init() {
	mainCommand.AddCommand(commandMerge)
}

func merge(outputPath string) error {
	mergedOptions, err := readConfigAndMerge()
	if err != nil {
		return err
	}
	err = mergePathResources(&mergedOptions)
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

func mergePathResources(options *option.Options) error {
	for _, inbound := range options.Inbounds {
		if tlsOptions, containsTLSOptions := inbound.Options.(option.InboundTLSOptionsWrapper); containsTLSOptions {
			tlsOptions.ReplaceInboundTLSOptions(mergeTLSInboundOptions(tlsOptions.TakeInboundTLSOptions()))
		}
	}
	for _, outbound := range options.Outbounds {
		switch outbound.Type {
		case C.TypeSSH:
			mergeSSHOutboundOptions(outbound.Options.(*option.SSHOutboundOptions))
		}
		if tlsOptions, containsTLSOptions := outbound.Options.(option.OutboundTLSOptionsWrapper); containsTLSOptions {
			tlsOptions.ReplaceOutboundTLSOptions(mergeTLSOutboundOptions(tlsOptions.TakeOutboundTLSOptions()))
		}
	}
	return nil
}

func mergeTLSInboundOptions(options *option.InboundTLSOptions) *option.InboundTLSOptions {
	if options == nil {
		return nil
	}
	if options.CertificatePath != "" {
		if content, err := os.ReadFile(options.CertificatePath); err == nil {
			options.Certificate = trimStringArray(strings.Split(string(content), "\n"))
		}
	}
	if options.KeyPath != "" {
		if content, err := os.ReadFile(options.KeyPath); err == nil {
			options.Key = trimStringArray(strings.Split(string(content), "\n"))
		}
	}
	if options.ECH != nil {
		if options.ECH.KeyPath != "" {
			if content, err := os.ReadFile(options.ECH.KeyPath); err == nil {
				options.ECH.Key = trimStringArray(strings.Split(string(content), "\n"))
			}
		}
	}
	return options
}

func mergeTLSOutboundOptions(options *option.OutboundTLSOptions) *option.OutboundTLSOptions {
	if options == nil {
		return nil
	}
	if options.CertificatePath != "" {
		if content, err := os.ReadFile(options.CertificatePath); err == nil {
			options.Certificate = trimStringArray(strings.Split(string(content), "\n"))
		}
	}
	if options.ECH != nil {
		if options.ECH.ConfigPath != "" {
			if content, err := os.ReadFile(options.ECH.ConfigPath); err == nil {
				options.ECH.Config = trimStringArray(strings.Split(string(content), "\n"))
			}
		}
	}
	return options
}

func mergeSSHOutboundOptions(options *option.SSHOutboundOptions) {
	if options.PrivateKeyPath != "" {
		if content, err := os.ReadFile(os.ExpandEnv(options.PrivateKeyPath)); err == nil {
			options.PrivateKey = trimStringArray(strings.Split(string(content), "\n"))
		}
	}
}

func trimStringArray(array []string) []string {
	return common.Filter(array, func(it string) bool {
		return strings.TrimSpace(it) != ""
	})
}
