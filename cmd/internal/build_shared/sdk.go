package build_shared

import (
	"go/build"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/rw"
)

var (
	androidSDKPath string
	androidNDKPath string
)

func FindSDK() {
	searchPath := []string{
		"$ANDROID_HOME",
		"$HOME/Android/Sdk",
		"$HOME/.local/lib/android/sdk",
		"$HOME/Library/Android/sdk",
	}
	for _, path := range searchPath {
		path = os.ExpandEnv(path)
		if rw.FileExists(path + "/licenses/android-sdk-license") {
			androidSDKPath = path
			break
		}
	}
	if androidSDKPath == "" {
		log.Fatal("android SDK not found")
	}
	if !findNDK() {
		log.Fatal("android NDK not found")
	}

	os.Setenv("ANDROID_HOME", androidSDKPath)
	os.Setenv("ANDROID_SDK_HOME", androidSDKPath)
	os.Setenv("ANDROID_NDK_HOME", androidNDKPath)
	os.Setenv("NDK", androidNDKPath)
	os.Setenv("PATH", os.Getenv("PATH")+":"+filepath.Join(androidNDKPath, "toolchains", "llvm", "prebuilt", runtime.GOOS+"-x86_64", "bin"))
}

func findNDK() bool {
	if rw.FileExists(androidSDKPath + "/ndk/25.1.8937393") {
		androidNDKPath = androidSDKPath + "/ndk/25.1.8937393"
		return true
	}
	ndkVersions, err := os.ReadDir(androidSDKPath + "/ndk")
	if err != nil {
		return false
	}
	versionNames := common.Map(ndkVersions, os.DirEntry.Name)
	if len(versionNames) == 0 {
		return false
	}
	sort.Slice(versionNames, func(i, j int) bool {
		iVersions := strings.Split(versionNames[i], ".")
		jVersions := strings.Split(versionNames[j], ".")
		for k := 0; k < len(iVersions) && k < len(jVersions); k++ {
			iVersion, _ := strconv.Atoi(iVersions[k])
			jVersion, _ := strconv.Atoi(jVersions[k])
			if iVersion != jVersion {
				return iVersion > jVersion
			}
		}
		return true
	})
	for _, versionName := range versionNames {
		if rw.FileExists(androidSDKPath + "/ndk/" + versionName) {
			androidNDKPath = androidSDKPath + "/ndk/" + versionName
			return true
		}
	}
	return false
}

var GoBinPath string

func FindMobile() {
	goBin := filepath.Join(build.Default.GOPATH, "bin")
	
	if runtime.GOOS == "windows" {
		if !rw.FileExists(goBin + "/" + "gobind.exe") {
			log.Fatal("missing gomobile.exe installation")
		}
	} else {
		if !rw.FileExists(goBin + "/" + "gobind") {
			log.Fatal("missing gomobile installation")
		}
	}
	GoBinPath = goBin
}
