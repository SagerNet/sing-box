package main

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sagernet/sing-box/cmd/internal/build_shared"
	"github.com/sagernet/sing-box/common/badversion"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"

	"howett.net/plist"
)

var (
	flagRunInCI    bool
	flagTestFlight bool
)

func init() {
	flag.BoolVar(&flagRunInCI, "ci", false, "Run in CI")
	flag.BoolVar(&flagTestFlight, "testflight", false, "Override the App Store marketing version with the version reserved for TestFlight")
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
	newContent := projectContent
	var marketingVersionUpdated bool
	if flagTestFlight {
		testFlightVersion := build_shared.TestFlightVersion(newVersion)
		newContent, marketingVersionUpdated = findAndReplace(objectsMap, newContent, []string{"io.nekohasekai.sfamt"}, testFlightVersion)
		if marketingVersionUpdated {
			log.Info("updated App Store version to ", testFlightVersion)
		}
	}
	var standaloneVersionUpdated bool
	newContent, standaloneVersionUpdated = findAndReplace(objectsMap, newContent, []string{"io.nekohasekai.sfamt.standalone", "io.nekohasekai.sfamt.system"}, newVersion.String())
	if standaloneVersionUpdated {
		marketingVersionUpdated = true
		log.Info("updated version to ", newVersion.String())
	}
	var projectVersionUpdated bool
	for environmentName, directory := range map[string]string{
		"IOS_PROJECT_VERSION":   "SFI",
		"MACOS_PROJECT_VERSION": "SFM",
		"TVOS_PROJECT_VERSION":  "SFT",
	} {
		projectVersion := os.Getenv(environmentName)
		if projectVersion == "" {
			continue
		}
		var updated bool
		newContent, updated = findAndReplaceProjectVersion(objectsMap, newContent, []string{directory}, projectVersion)
		if updated {
			projectVersionUpdated = true
			log.Info("updated ", directory, " project version to ", projectVersion)
		}
	}
	if marketingVersionUpdated || projectVersionUpdated {
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
		version := strings.Trim(projectContent[versionStart:versionEnd], "\"")
		if version == newVersion {
			continue
		}
		updated = true
		projectContent = projectContent[:versionStart] + formatProjectVersion(newVersion) + projectContent[versionEnd:]
	}
	return projectContent, updated
}

// Xcode serializes a version without quotes unless it contains a pre-release
// part; always quoting makes Xcode rewrite the value on the next save.
func formatProjectVersion(version string) string {
	if badversion.Parse(version).PreReleaseIdentifier == "" {
		return version
	}
	return "\"" + version + "\""
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
	globalSettings := collectBuildSettings(objectsMap)
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
		bundleID := expandBuildVariables(bundleIDObject.(string), globalSettings)
		if common.Contains(bundleIDList, bundleID) {
			objectKeyList = append(objectKeyList, objectKey)
		}
	}
	return objectKeyList
}

func collectBuildSettings(objectsMap map[string]any) map[string]string {
	settings := make(map[string]string)
	for _, object := range objectsMap {
		buildSettings, loaded := object.(map[string]any)["buildSettings"].(map[string]any)
		if !loaded {
			continue
		}
		for key, value := range buildSettings {
			valueString, isString := value.(string)
			if !isString {
				continue
			}
			settings[key] = valueString
		}
	}
	return settings
}

var buildVariableRegexp = regexp.MustCompile(`\$[({]([A-Za-z0-9_]+)[)}]`)

func expandBuildVariables(value string, settings map[string]string) string {
	for {
		expanded := buildVariableRegexp.ReplaceAllStringFunc(value, func(match string) string {
			name := buildVariableRegexp.FindStringSubmatch(match)[1]
			replacement, loaded := settings[name]
			if !loaded {
				return match
			}
			return replacement
		})
		if expanded == value {
			return expanded
		}
		value = expanded
	}
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
