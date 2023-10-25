//go:build android

package libbox

import (
	"archive/zip"
	"bytes"
	"debug/buildinfo"
	"io"
	"runtime/debug"
	"strings"

	"github.com/sagernet/sing/common"
)

const (
	androidVPNCoreTypeOpenVPN     = "OpenVPN"
	androidVPNCoreTypeShadowsocks = "Shadowsocks"
	androidVPNCoreTypeClash       = "Clash"
	androidVPNCoreTypeV2Ray       = "V2Ray"
	androidVPNCoreTypeWireGuard   = "WireGuard"
	androidVPNCoreTypeSingBox     = "sing-box"
	androidVPNCoreTypeUnknown     = "Unknown"
)

type AndroidVPNType struct {
	CoreType  string
	CorePath  string
	GoVersion string
}

func ReadAndroidVPNType(publicSourceDirList StringIterator) (*AndroidVPNType, error) {
	apkPathList := iteratorToArray[string](publicSourceDirList)
	var lastError error
	for _, apkPath := range apkPathList {
		androidVPNType, err := readAndroidVPNType(apkPath)
		if androidVPNType == nil {
			if err != nil {
				lastError = err
			}
			continue
		}
		return androidVPNType, nil
	}
	return nil, lastError
}

func readAndroidVPNType(publicSourceDir string) (*AndroidVPNType, error) {
	reader, err := zip.OpenReader(publicSourceDir)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	var lastError error
	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, "lib/") {
			continue
		}
		vpnType, err := readAndroidVPNTypeEntry(file)
		if err != nil {
			lastError = err
			continue
		}
		return vpnType, nil
	}
	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, "lib/") {
			continue
		}
		if strings.Contains(file.Name, androidVPNCoreTypeOpenVPN) || strings.Contains(file.Name, "ovpn") {
			return &AndroidVPNType{CoreType: androidVPNCoreTypeOpenVPN}, nil
		}
		if strings.Contains(file.Name, androidVPNCoreTypeShadowsocks) {
			return &AndroidVPNType{CoreType: androidVPNCoreTypeShadowsocks}, nil
		}
	}
	return nil, lastError
}

func readAndroidVPNTypeEntry(zipFile *zip.File) (*AndroidVPNType, error) {
	readCloser, err := zipFile.Open()
	if err != nil {
		return nil, err
	}
	libContent := make([]byte, zipFile.UncompressedSize64)
	_, err = io.ReadFull(readCloser, libContent)
	readCloser.Close()
	if err != nil {
		return nil, err
	}
	buildInfo, err := buildinfo.Read(bytes.NewReader(libContent))
	if err != nil {
		return nil, err
	}
	var vpnType AndroidVPNType
	vpnType.GoVersion = buildInfo.GoVersion
	if !strings.HasPrefix(vpnType.GoVersion, "go") {
		vpnType.GoVersion = "obfuscated"
	} else {
		vpnType.GoVersion = vpnType.GoVersion[2:]
	}
	vpnType.CoreType = androidVPNCoreTypeUnknown
	if len(buildInfo.Deps) == 0 {
		vpnType.CoreType = "obfuscated"
		return &vpnType, nil
	}

	dependencies := make(map[string]bool)
	dependencies[buildInfo.Path] = true
	for _, module := range buildInfo.Deps {
		dependencies[module.Path] = true
		if module.Replace != nil {
			dependencies[module.Replace.Path] = true
		}
	}
	for dependency := range dependencies {
		pkgType, loaded := determinePkgType(dependency)
		if loaded {
			vpnType.CoreType = pkgType
		}
	}
	if vpnType.CoreType == androidVPNCoreTypeUnknown {
		for dependency := range dependencies {
			pkgType, loaded := determinePkgTypeSecondary(dependency)
			if loaded {
				vpnType.CoreType = pkgType
				return &vpnType, nil
			}
		}
	}
	if vpnType.CoreType != androidVPNCoreTypeUnknown {
		vpnType.CorePath, _ = determineCorePath(buildInfo, vpnType.CoreType)
		return &vpnType, nil
	}
	if dependencies["github.com/golang/protobuf"] && dependencies["github.com/v2fly/ss-bloomring"] {
		vpnType.CoreType = androidVPNCoreTypeV2Ray
		return &vpnType, nil
	}
	return &vpnType, nil
}

func determinePkgType(pkgName string) (string, bool) {
	pkgNameLower := strings.ToLower(pkgName)
	if strings.Contains(pkgNameLower, "clash") {
		return androidVPNCoreTypeClash, true
	}
	if strings.Contains(pkgNameLower, "v2ray") || strings.Contains(pkgNameLower, "xray") {
		return androidVPNCoreTypeV2Ray, true
	}

	if strings.Contains(pkgNameLower, "sing-box") {
		return androidVPNCoreTypeSingBox, true
	}
	return "", false
}

func determinePkgTypeSecondary(pkgName string) (string, bool) {
	pkgNameLower := strings.ToLower(pkgName)
	if strings.Contains(pkgNameLower, "wireguard") {
		return androidVPNCoreTypeWireGuard, true
	}
	return "", false
}

func determineCorePath(pkgInfo *buildinfo.BuildInfo, pkgType string) (string, bool) {
	switch pkgType {
	case androidVPNCoreTypeClash:
		return determineCorePathForPkgs(pkgInfo, []string{"github.com/Dreamacro/clash"}, []string{"clash"})
	case androidVPNCoreTypeV2Ray:
		if v2rayVersion, loaded := determineCorePathForPkgs(pkgInfo, []string{
			"github.com/v2fly/v2ray-core",
			"github.com/v2fly/v2ray-core/v4",
			"github.com/v2fly/v2ray-core/v5",
		}, []string{
			"v2ray",
		}); loaded {
			return v2rayVersion, true
		}
		if xrayVersion, loaded := determineCorePathForPkgs(pkgInfo, []string{
			"github.com/xtls/xray-core",
		}, []string{
			"xray",
		}); loaded {
			return xrayVersion, true
		}
		return "", false
	case androidVPNCoreTypeSingBox:
		return determineCorePathForPkgs(pkgInfo, []string{"github.com/sagernet/sing-box"}, []string{"sing-box"})
	case androidVPNCoreTypeWireGuard:
		return determineCorePathForPkgs(pkgInfo, []string{"golang.zx2c4.com/wireguard"}, []string{"wireguard"})
	default:
		return "", false
	}
}

func determineCorePathForPkgs(pkgInfo *buildinfo.BuildInfo, pkgs []string, names []string) (string, bool) {
	for _, pkg := range pkgs {
		if pkgInfo.Path == pkg {
			return pkg, true
		}
		strictDependency := common.Find(pkgInfo.Deps, func(module *debug.Module) bool {
			return module.Path == pkg
		})
		if strictDependency != nil {
			if isValidVersion(strictDependency.Version) {
				return strictDependency.Path + " " + strictDependency.Version, true
			} else {
				return strictDependency.Path, true
			}
		}
	}
	for _, name := range names {
		if strings.Contains(pkgInfo.Path, name) {
			return pkgInfo.Path, true
		}
		looseDependency := common.Find(pkgInfo.Deps, func(module *debug.Module) bool {
			return strings.Contains(module.Path, name) || (module.Replace != nil && strings.Contains(module.Replace.Path, name))
		})
		if looseDependency != nil {
			return looseDependency.Path, true
		}
	}
	return "", false
}

func isValidVersion(version string) bool {
	if version == "(devel)" {
		return false
	}
	if strings.Contains(version, "v0.0.0") {
		return false
	}
	return true
}
