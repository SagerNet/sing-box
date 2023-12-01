package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/sagernet/sing-box/common/json"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/rw"

	"github.com/spf13/cobra"
)

var commandMerge = &cobra.Command{
	Use:   "merge <output>",
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
	err = rw.WriteFile(outputPath, buffer.Bytes())
	if err != nil {
		return err
	}
	outputPath, _ = filepath.Abs(outputPath)
	os.Stderr.WriteString(outputPath + "\n")
	return nil
}

func mergePathResources(options *option.Options) error {
	for index, inbound := range options.Inbounds {
		switch inbound.Type {
		case C.TypeHTTP:
			inbound.HTTPOptions.TLS = mergeTLSInboundOptions(inbound.HTTPOptions.TLS)
		case C.TypeMixed:
			inbound.MixedOptions.TLS = mergeTLSInboundOptions(inbound.MixedOptions.TLS)
		case C.TypeVMess:
			inbound.VMessOptions.TLS = mergeTLSInboundOptions(inbound.VMessOptions.TLS)
		case C.TypeTrojan:
			inbound.TrojanOptions.TLS = mergeTLSInboundOptions(inbound.TrojanOptions.TLS)
		case C.TypeNaive:
			inbound.NaiveOptions.TLS = mergeTLSInboundOptions(inbound.NaiveOptions.TLS)
		case C.TypeHysteria:
			inbound.HysteriaOptions.TLS = mergeTLSInboundOptions(inbound.HysteriaOptions.TLS)
		case C.TypeVLESS:
			inbound.VLESSOptions.TLS = mergeTLSInboundOptions(inbound.VLESSOptions.TLS)
		case C.TypeTUIC:
			inbound.TUICOptions.TLS = mergeTLSInboundOptions(inbound.TUICOptions.TLS)
		case C.TypeHysteria2:
			inbound.Hysteria2Options.TLS = mergeTLSInboundOptions(inbound.Hysteria2Options.TLS)
		default:
			continue
		}
		options.Inbounds[index] = inbound
	}
	for index, outbound := range options.Outbounds {
		switch outbound.Type {
		case C.TypeHTTP:
			outbound.HTTPOptions.TLS = mergeTLSOutboundOptions(outbound.HTTPOptions.TLS)
		case C.TypeVMess:
			outbound.VMessOptions.TLS = mergeTLSOutboundOptions(outbound.VMessOptions.TLS)
		case C.TypeTrojan:
			outbound.TrojanOptions.TLS = mergeTLSOutboundOptions(outbound.TrojanOptions.TLS)
		case C.TypeHysteria:
			outbound.HysteriaOptions.TLS = mergeTLSOutboundOptions(outbound.HysteriaOptions.TLS)
		case C.TypeSSH:
			outbound.SSHOptions = mergeSSHOutboundOptions(outbound.SSHOptions)
		case C.TypeVLESS:
			outbound.VLESSOptions.TLS = mergeTLSOutboundOptions(outbound.VLESSOptions.TLS)
		case C.TypeTUIC:
			outbound.TUICOptions.TLS = mergeTLSOutboundOptions(outbound.TUICOptions.TLS)
		case C.TypeHysteria2:
			outbound.Hysteria2Options.TLS = mergeTLSOutboundOptions(outbound.Hysteria2Options.TLS)
		default:
			continue
		}
		options.Outbounds[index] = outbound
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

func mergeSSHOutboundOptions(options option.SSHOutboundOptions) option.SSHOutboundOptions {
	if options.PrivateKeyPath != "" {
		if content, err := os.ReadFile(os.ExpandEnv(options.PrivateKeyPath)); err == nil {
			options.PrivateKey = trimStringArray(strings.Split(string(content), "\n"))
		}
	}
	return options
}

func trimStringArray(array []string) []string {
	return common.Filter(array, func(it string) bool {
		return strings.TrimSpace(it) != ""
	})
}
