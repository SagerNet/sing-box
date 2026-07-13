package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"

	"github.com/sagernet/sing-box/cmd/internal/build_shared"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
)

var (
	flagRunInCI    bool
	flagRunNightly bool
)

type versionMetadata struct {
	Version string `json:"version"`
}

func init() {
	flag.BoolVar(&flagRunInCI, "ci", false, "Run in CI")
	flag.BoolVar(&flagRunNightly, "nightly", false, "Run nightly")
}

func main() {
	flag.Parse()
	newVersion := common.Must1(build_shared.ReadTag())
	desktopPath := "../sing-box-for-desktop"
	if flagRunInCI {
		desktopPath = "clients/desktop"
	}
	desktopPath = common.Must1(filepath.Abs(desktopPath))
	versionPath := filepath.Join(desktopPath, "version.json")
	versionFile := common.Must1(os.Open(versionPath))
	var metadata versionMetadata
	common.Must(json.NewDecoder(versionFile).Decode(&metadata))
	common.Must(versionFile.Close())
	if metadata.Version == newVersion {
		log.Info("version not changed")
		return
	}
	log.Info("updated version from ", metadata.Version, " to ", newVersion)
	if flagRunInCI && !flagRunNightly {
		log.Fatal("version changed, commit changes first.")
	}
	metadata.Version = newVersion
	outputFile := common.Must1(os.Create(versionPath))
	encoder := json.NewEncoder(outputFile)
	encoder.SetIndent("", "  ")
	common.Must(encoder.Encode(metadata))
	common.Must(outputFile.Close())
}
