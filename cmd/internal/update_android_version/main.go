package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/cmd/internal/build_shared"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
)

func main() {
	newTag := common.Must1(build_shared.ReadTag())
	androidPath, err := filepath.Abs("../sing-box-for-android")
	if err != nil {
		log.Fatal(err)
	}
	common.Must(os.Chdir(androidPath))
	localProps := common.Must1(os.ReadFile("local.properties"))
	var propsList [][]string
	for _, propLine := range strings.Split(string(localProps), "\n") {
		propsList = append(propsList, strings.Split(propLine, "="))
	}
	for _, propPair := range propsList {
		if propPair[0] == "VERSION_NAME" {
			if propPair[1] == newTag {
				log.Info("version not changed")
				return
			}
			propPair[1] = newTag
			log.Info("updated version to ", newTag)
		}
	}
	for _, propPair := range propsList {
		switch propPair[0] {
		case "VERSION_CODE":
			versionCode := common.Must1(strconv.ParseInt(propPair[1], 10, 64))
			propPair[1] = strconv.Itoa(int(versionCode + 1))
			log.Info("updated version code to ", propPair[1])
		case "RELEASE_NOTES":
			propPair[1] = "sing-box " + newTag
		}
	}
	var newProps []string
	for _, propPair := range propsList {
		newProps = append(newProps, strings.Join(propPair, "="))
	}
	common.Must(os.WriteFile("local.properties", []byte(strings.Join(newProps, "\n")), 0o644))
}
