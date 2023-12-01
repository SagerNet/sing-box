package main

import (
	"github.com/sagernet/sing-box/common/geosite"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var (
	commandGeoSiteFlagFile string
	geositeReader          *geosite.Reader
	geositeCodeList        []string
)

var commandGeoSite = &cobra.Command{
	Use:   "geosite",
	Short: "Geosite tools",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		err := geositePreRun()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandGeoSite.PersistentFlags().StringVarP(&commandGeoSiteFlagFile, "file", "f", "geosite.db", "geosite file")
	mainCommand.AddCommand(commandGeoSite)
}

func geositePreRun() error {
	reader, codeList, err := geosite.Open(commandGeoSiteFlagFile)
	if err != nil {
		return E.Cause(err, "open geosite file")
	}
	geositeReader = reader
	geositeCodeList = codeList
	return nil
}
