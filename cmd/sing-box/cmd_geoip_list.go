package main

import (
	"os"

	"github.com/sagernet/sing-box/log"

	"github.com/spf13/cobra"
)

var commandGeoipList = &cobra.Command{
	Use:   "list",
	Short: "List geoip country codes",
	Run: func(cmd *cobra.Command, args []string) {
		err := listGeoip()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandGeoip.AddCommand(commandGeoipList)
}

func listGeoip() error {
	for _, code := range geoipReader.Metadata.Languages {
		os.Stdout.WriteString(code + "\n")
	}
	return nil
}
