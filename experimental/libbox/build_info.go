//go:build android

package libbox

import (
	"archive/zip"
	"bytes"
	"debug/buildinfo"
	"io"
	"strings"
)

const (
	AndroidVPNCoreTypeOpenVPN     = "OpenVPN"
	AndroidVPNCoreTypeShadowsocks = "Shadowsocks"
	AndroidVPNCoreTypeClash       = "Clash"
	AndroidVPNCoreTypeV2Ray       = "V2Ray"
	AndroidVPNCoreTypeWireGuard   = "WireGuard"
	AndroidVPNCoreTypeSingBox     = "sing-box"
	AndroidVPNCoreTypeUnknown     = "Unknown"
)

type AndroidVPNType struct {
	CoreType  string
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
		if strings.Contains(file.Name, AndroidVPNCoreTypeOpenVPN) || strings.Contains(file.Name, "ovpn") {
			return &AndroidVPNType{AndroidVPNCoreTypeOpenVPN, ""}, nil
		}
		if strings.Contains(file.Name, AndroidVPNCoreTypeShadowsocks) {
			return &AndroidVPNType{AndroidVPNCoreTypeShadowsocks, ""}, nil
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
	vpnType.CoreType = AndroidVPNCoreTypeUnknown
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
			return &vpnType, nil
		}
	}
	for dependency := range dependencies {
		pkgType, loaded := determinePkgTypeSecondary(dependency)
		if loaded {
			vpnType.CoreType = pkgType
			return &vpnType, nil
		}
	}
	if dependencies["github.com/golang/protobuf"] && dependencies["github.com/v2fly/ss-bloomring"] {
		vpnType.CoreType = AndroidVPNCoreTypeV2Ray
		return &vpnType, nil
	}
	return &vpnType, nil
}

func determinePkgType(pkgName string) (string, bool) {
	pkgNameLower := strings.ToLower(pkgName)
	if strings.Contains(pkgNameLower, "clash") {
		return AndroidVPNCoreTypeClash, true
	}
	if strings.Contains(pkgNameLower, "v2ray") || strings.Contains(pkgNameLower, "xray") {
		return AndroidVPNCoreTypeV2Ray, true
	}

	if strings.Contains(pkgNameLower, "sing-box") {
		return AndroidVPNCoreTypeSingBox, true
	}
	return "", false
}

func determinePkgTypeSecondary(pkgName string) (string, bool) {
	pkgNameLower := strings.ToLower(pkgName)
	if strings.Contains(pkgNameLower, "wireguard") {
		return AndroidVPNCoreTypeWireGuard, true
	}
	return "", false
}
