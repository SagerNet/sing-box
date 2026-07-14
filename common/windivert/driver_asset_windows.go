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

	"golang.org/x/sys/windows"
)

func extractVerified() (string, *os.File, error) {
	if len(sysBytes) == 0 {
		return "", nil, E.New("windivert: unsupported architecture ", runtime.GOARCH)
	}

	base, err := os.UserCacheDir()
	if err != nil {
		return "", nil, E.Cause(err, "windivert: locate user cache dir")
	}
	dir := filepath.Join(base, "sing-box", "windivert", "v"+AssetVersion)
	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		return "", nil, E.Cause(err, "windivert: mkdir ", dir)
	}
	target := filepath.Join(dir, driverSysName())

	for attempt := 0; ; attempt++ {
		sysFile, err := openDriverFile(target)
		if err != nil {
			if !os.IsNotExist(err) {
				return "", nil, E.Cause(err, "windivert: open ", target)
			}
			err = writeDriverFile(target)
			if err != nil {
				return "", nil, err
			}
			sysFile, err = openDriverFile(target)
			if err != nil {
				return "", nil, E.Cause(err, "windivert: open ", target)
			}
		}
		content, err := io.ReadAll(sysFile)
		if err != nil {
			sysFile.Close()
			return "", nil, E.Cause(err, "windivert: read ", target)
		}
		if bytes.Equal(content, sysBytes) {
			return target, sysFile, nil
		}
		sysFile.Close()
		if attempt > 0 {
			return "", nil, E.New("windivert: driver file ", target, " is being concurrently modified")
		}
		err = writeDriverFile(target)
		if err != nil {
			return "", nil, err
		}
	}
}

func openDriverFile(path string) (*os.File, error) {
	pathW, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		pathW,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}

func writeDriverFile(target string) error {
	temporaryPath := target + ".tmp-" + strconv.Itoa(os.Getpid())
	err := os.WriteFile(temporaryPath, sysBytes, 0o644)
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
