package main

import (
	"net/netip"
	"os"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"

	"github.com/spf13/cobra"
)

var commandGeoipLookup = &cobra.Command{
	Use:   "lookup <address>",
	Short: "Lookup if an IP address is contained in the GeoIP database",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := geoipLookup(args[0])
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandGeoip.AddCommand(commandGeoipLookup)
}

func geoipLookup(address string) error {
	addr, err := netip.ParseAddr(address)
	if err != nil {
		return E.Cause(err, "parse address")
	}
	if !N.IsPublicAddr(addr) {
		os.Stdout.WriteString("private\n")
		return nil
	}
	var code string
	_ = geoipReader.Lookup(addr.AsSlice(), &code)
	if code != "" {
		os.Stdout.WriteString(code + "\n")
		return nil
	}
	os.Stdout.WriteString("unknown\n")
	return nil
}
