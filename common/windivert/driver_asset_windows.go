//go:build windows

package windivert

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	E "github.com/sagernet/sing/common/exceptions"
)

func driverFilePath() (string, error) {
	if driverAssetName == "" {
		return "", E.New("windivert: unsupported architecture ", runtime.GOARCH)
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", E.Cause(err, "windivert: locate user cache dir")
	}
	return filepath.Join(base, "sing-box", "windivert", "v"+AssetVersion, driverAssetName), nil
}

func openVerifiedDriver() (string, *os.File, error) {
	target, err := driverFilePath()
	if err != nil {
		return "", nil, err
	}
	assetContent, err := driverAsset()
	if err != nil {
		return "", nil, err
	}
	err = os.MkdirAll(filepath.Dir(target), 0o755)
	if err != nil {
		return "", nil, E.Cause(err, "windivert: mkdir ", filepath.Dir(target))
	}

	var (
		sysFile *os.File
		content []byte
	)
	for attempt := 0; ; attempt++ {
		sysFile, err = openDriverFile(target)
		if err != nil {
			if !os.IsNotExist(err) {
				return "", nil, E.Cause(err, "windivert: open ", target)
			}
			err = writeDriverFile(target, assetContent)
			if err != nil {
				return "", nil, err
			}
			sysFile, err = openDriverFile(target)
			if err != nil {
				return "", nil, E.Cause(err, "windivert: open ", target)
			}
		}
		content, err = io.ReadAll(sysFile)
		if err != nil {
			sysFile.Close()
			return "", nil, E.Cause(err, "windivert: read ", target)
		}
		if bytes.Equal(content, assetContent) {
			return target, sysFile, nil
		}
		sysFile.Close()
		if attempt > 0 {
			return "", nil, E.New("windivert: driver file ", target, " is being concurrently modified")
		}
		err = writeDriverFile(target, assetContent)
		if err != nil {
			return "", nil, err
		}
	}
}

func writeDriverFile(target string, content []byte) error {
	temporaryPath := target + ".tmp-" + strconv.Itoa(os.Getpid())
	err := os.WriteFile(temporaryPath, content, 0o644)
	if err != nil {
		return E.Cause(err, "windivert: write ", filepath.Base(target))
	}
	err = os.Rename(temporaryPath, target)
	if err != nil {
		os.Remove(temporaryPath)
		return E.Cause(err, "windivert: rename ", filepath.Base(target))
	}
	return nil
}
