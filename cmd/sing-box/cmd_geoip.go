package main

import (
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/oschwald/maxminddb-golang"
	"github.com/spf13/cobra"
)

var (
	geoipReader          *maxminddb.Reader
	commandGeoIPFlagFile string
)

var commandGeoip = &cobra.Command{
	Use:   "geoip",
	Short: "GeoIP tools",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		err := geoipPreRun()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandGeoip.PersistentFlags().StringVarP(&commandGeoIPFlagFile, "file", "f", "geoip.db", "geoip file")
	mainCommand.AddCommand(commandGeoip)
}

func geoipPreRun() error {
	reader, err := maxminddb.Open(commandGeoIPFlagFile)
	if err != nil {
		return err
	}
	if reader.Metadata.DatabaseType != "sing-geoip" {
		reader.Close()
		return E.New("incorrect database type, expected sing-geoip, got ", reader.Metadata.DatabaseType)
	}
	geoipReader = reader
	return nil
}
