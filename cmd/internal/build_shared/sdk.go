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
		if rw.IsFile(filepath.Join(path, "licenses", "android-sdk-license")) {
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
	const fixedVersion = "28.0.13004108"
	const versionFile = "source.properties"
	if fixedPath := filepath.Join(androidSDKPath, "ndk", fixedVersion); rw.IsFile(filepath.Join(fixedPath, versionFile)) {
		androidNDKPath = fixedPath
		return true
	}
	if ndkHomeEnv := os.Getenv("ANDROID_NDK_HOME"); rw.IsFile(filepath.Join(ndkHomeEnv, versionFile)) {
		androidNDKPath = ndkHomeEnv
		return true
	}
	ndkVersions, err := os.ReadDir(filepath.Join(androidSDKPath, "ndk"))
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
		currentNDKPath := filepath.Join(androidSDKPath, "ndk", versionName)
		if rw.IsFile(filepath.Join(currentNDKPath, versionFile)) {
			androidNDKPath = currentNDKPath
			log.Warn("reproducibility warning: using NDK version " + versionName + " instead of " + fixedVersion)
			return true
		}
	}
	return false
}

var GoBinPath string

func FindMobile() {
	goBin := filepath.Join(build.Default.GOPATH, "bin")
	if runtime.GOOS == "windows" {
		if !rw.IsFile(filepath.Join(goBin, "gobind.exe")) {
			log.Fatal("missing gomobile installation")
		}
	} else {
		if !rw.IsFile(filepath.Join(goBin, "gobind")) {
			log.Fatal("missing gomobile installation")
		}
	}
	GoBinPath = goBin
}
