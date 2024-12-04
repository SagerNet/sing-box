package main

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sagernet/sing-box/cmd/internal/build_shared"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"

	"howett.net/plist"
)

var flagRunInCI bool

func init() {
	flag.BoolVar(&flagRunInCI, "ci", false, "Run in CI")
}

func main() {
	flag.Parse()
	newVersion := common.Must1(build_shared.ReadTagVersion())
	var applePath string
	if flagRunInCI {
		applePath = "clients/apple"
	} else {
		applePath = "../sing-box-for-apple"
	}
	applePath, err := filepath.Abs(applePath)
	if err != nil {
		log.Fatal(err)
	}
	common.Must(os.Chdir(applePath))
	projectFile := common.Must1(os.Open("sing-box.xcodeproj/project.pbxproj"))
	var project map[string]any
	decoder := plist.NewDecoder(projectFile)
	common.Must(decoder.Decode(&project))
	objectsMap := project["objects"].(map[string]any)
	projectContent := string(common.Must1(os.ReadFile("sing-box.xcodeproj/project.pbxproj")))
	newContent, updated0 := findAndReplace(objectsMap, projectContent, []string{"io.nekohasekai.sfavt"}, newVersion.VersionString())
	newContent, updated1 := findAndReplace(objectsMap, newContent, []string{"io.nekohasekai.sfavt.standalone", "io.nekohasekai.sfavt.system"}, newVersion.String())
	if updated0 || updated1 {
		log.Info("updated version to ", newVersion.VersionString(), " (", newVersion.String(), ")")
	}
	var updated2 bool
	if macProjectVersion := os.Getenv("MACOS_PROJECT_VERSION"); macProjectVersion != "" {
		newContent, updated2 = findAndReplaceProjectVersion(objectsMap, newContent, []string{"SFM"}, macProjectVersion)
		if updated2 {
			log.Info("updated macos project version to ", macProjectVersion)
		}
	}
	if updated0 || updated1 || updated2 {
		common.Must(os.WriteFile("sing-box.xcodeproj/project.pbxproj", []byte(newContent), 0o644))
	}
}

func findAndReplace(objectsMap map[string]any, projectContent string, bundleIDList []string, newVersion string) (string, bool) {
	objectKeyList := findObjectKey(objectsMap, bundleIDList)
	var updated bool
	for _, objectKey := range objectKeyList {
		matchRegexp := common.Must1(regexp.Compile(objectKey + ".*= \\{"))
		indexes := matchRegexp.FindStringIndex(projectContent)
		if len(indexes) < 2 {
			println(projectContent)
			log.Fatal("failed to find object key ", objectKey, ": ", strings.Index(projectContent, objectKey))
		}
		indexStart := indexes[1]
		indexEnd := indexStart + strings.Index(projectContent[indexStart:], "}")
		versionStart := indexStart + strings.Index(projectContent[indexStart:indexEnd], "MARKETING_VERSION = ") + 20
		versionEnd := versionStart + strings.Index(projectContent[versionStart:indexEnd], ";")
		version := projectContent[versionStart:versionEnd]
		if version == newVersion {
			continue
		}
		updated = true
		projectContent = projectContent[:versionStart] + newVersion + projectContent[versionEnd:]
	}
	return projectContent, updated
}

func findAndReplaceProjectVersion(objectsMap map[string]any, projectContent string, directoryList []string, newVersion string) (string, bool) {
	objectKeyList := findObjectKeyByDirectory(objectsMap, directoryList)
	var updated bool
	for _, objectKey := range objectKeyList {
		matchRegexp := common.Must1(regexp.Compile(objectKey + ".*= \\{"))
		indexes := matchRegexp.FindStringIndex(projectContent)
		if len(indexes) < 2 {
			println(projectContent)
			log.Fatal("failed to find object key ", objectKey, ": ", strings.Index(projectContent, objectKey))
		}
		indexStart := indexes[1]
		indexEnd := indexStart + strings.Index(projectContent[indexStart:], "}")
		versionStart := indexStart + strings.Index(projectContent[indexStart:indexEnd], "CURRENT_PROJECT_VERSION = ") + 26
		versionEnd := versionStart + strings.Index(projectContent[versionStart:indexEnd], ";")
		version := projectContent[versionStart:versionEnd]
		if version == newVersion {
			continue
		}
		updated = true
		projectContent = projectContent[:versionStart] + newVersion + projectContent[versionEnd:]
	}
	return projectContent, updated
}

func findObjectKey(objectsMap map[string]any, bundleIDList []string) []string {
	var objectKeyList []string
	for objectKey, object := range objectsMap {
		buildSettings := object.(map[string]any)["buildSettings"]
		if buildSettings == nil {
			continue
		}
		bundleIDObject := buildSettings.(map[string]any)["PRODUCT_BUNDLE_IDENTIFIER"]
		if bundleIDObject == nil {
			continue
		}
		if common.Contains(bundleIDList, bundleIDObject.(string)) {
			objectKeyList = append(objectKeyList, objectKey)
		}
	}
	return objectKeyList
}

func findObjectKeyByDirectory(objectsMap map[string]any, directoryList []string) []string {
	var objectKeyList []string
	for objectKey, object := range objectsMap {
		buildSettings := object.(map[string]any)["buildSettings"]
		if buildSettings == nil {
			continue
		}
		infoPListFile := buildSettings.(map[string]any)["INFOPLIST_FILE"]
		if infoPListFile == nil {
			continue
		}
		for _, searchDirectory := range directoryList {
			if strings.HasPrefix(infoPListFile.(string), searchDirectory+"/") {
				objectKeyList = append(objectKeyList, objectKey)
			}
		}

	}
	return objectKeyList
}
