package main

import (
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/cmd/internal/build_shared"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
)

var flagRunInCI bool

func init() {
	flag.BoolVar(&flagRunInCI, "ci", false, "Run in CI")
}

func main() {
	flag.Parse()
	newVersion := common.Must1(build_shared.ReadTag())
	var androidPath string
	if flagRunInCI {
		androidPath = "clients/android"
	} else {
		androidPath = "../sing-box-for-android"
	}
	androidPath, err := filepath.Abs(androidPath)
	if err != nil {
		log.Fatal(err)
	}
	common.Must(os.Chdir(androidPath))
	localProps := common.Must1(os.ReadFile("version.properties"))
	var propsList [][]string
	for _, propLine := range strings.Split(string(localProps), "\n") {
		propsList = append(propsList, strings.Split(propLine, "="))
	}
	var (
		versionUpdated   bool
		goVersionUpdated bool
	)
	for _, propPair := range propsList {
		switch propPair[0] {
		case "VERSION_NAME":
			if propPair[1] != newVersion {
				versionUpdated = true
				propPair[1] = newVersion
				log.Info("updated version to ", newVersion)
			}
		case "GO_VERSION":
			if propPair[1] != runtime.Version() {
				goVersionUpdated = true
				propPair[1] = runtime.Version()
				log.Info("updated Go version to ", runtime.Version())
			}
		}
	}
	if !(versionUpdated || goVersionUpdated) {
		log.Info("version not changed")
		return
	}
	for _, propPair := range propsList {
		switch propPair[0] {
		case "VERSION_CODE":
			versionCode := common.Must1(strconv.ParseInt(propPair[1], 10, 64))
			propPair[1] = strconv.Itoa(int(versionCode + 1))
			log.Info("updated version code to ", propPair[1])
		}
	}
	var newProps []string
	for _, propPair := range propsList {
		newProps = append(newProps, strings.Join(propPair, "="))
	}
	common.Must(os.WriteFile("version.properties", []byte(strings.Join(newProps, "\n")), 0o644))
}
