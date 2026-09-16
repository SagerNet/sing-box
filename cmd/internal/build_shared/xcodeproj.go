package build_shared

import (
	"os"
	"path/filepath"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"

	"howett.net/plist"
)

func ReadAppleMarketingVersion(applePath string, directory string) (string, error) {
	projectFile, err := os.Open(filepath.Join(applePath, "sing-box.xcodeproj", "project.pbxproj"))
	if err != nil {
		return "", err
	}
	defer projectFile.Close()
	var project map[string]any
	err = plist.NewDecoder(projectFile).Decode(&project)
	if err != nil {
		return "", err
	}
	var marketingVersion string
	for _, object := range project["objects"].(map[string]any) {
		buildSettings, isMap := object.(map[string]any)["buildSettings"].(map[string]any)
		if !isMap {
			continue
		}
		infoPlistFile, isString := buildSettings["INFOPLIST_FILE"].(string)
		if !isString || !strings.HasPrefix(infoPlistFile, directory+"/") {
			continue
		}
		version, isString := buildSettings["MARKETING_VERSION"].(string)
		if !isString {
			continue
		}
		if marketingVersion != "" && marketingVersion != version {
			return "", E.New("inconsistent marketing versions in ", directory, ": ", marketingVersion, ", ", version)
		}
		marketingVersion = version
	}
	if marketingVersion == "" {
		return "", E.New("marketing version not found in ", directory)
	}
	return marketingVersion, nil
}
