package main

import (
	"os"
	"sort"

	"github.com/sagernet/sing-box/log"
	F "github.com/sagernet/sing/common/format"

	"github.com/spf13/cobra"
)

var commandGeositeList = &cobra.Command{
	Use:   "list <category>",
	Short: "List geosite categories",
	Run: func(cmd *cobra.Command, args []string) {
		err := geositeList()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandGeoSite.AddCommand(commandGeositeList)
}

func geositeList() error {
	var geositeEntry []struct {
		category string
		items    int
	}
	for _, category := range geositeCodeList {
		sourceSet, err := geositeReader.Read(category)
		if err != nil {
			return err
		}
		geositeEntry = append(geositeEntry, struct {
			category string
			items    int
		}{category, len(sourceSet)})
	}
	sort.SliceStable(geositeEntry, func(i, j int) bool {
		return geositeEntry[i].items < geositeEntry[j].items
	})
	for _, entry := range geositeEntry {
		os.Stdout.WriteString(F.ToString(entry.category, " (", entry.items, ")\n"))
	}
	return nil
}
