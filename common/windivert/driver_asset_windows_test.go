//go:build windows && !with_external_windivert

package windivert

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// The image lock on a loaded .sys can outlive SERVICE_STOPPED by tens of
// seconds (observed on GitHub-hosted runners), and on current runner images
// it blocks renames as well as writes and deletes.
func setTempDriverCache(t *testing.T) {
	t.Helper()
	dir, err := os.MkdirTemp("", "sing-box-windivert-test-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("LocalAppData", dir)
}

func TestIntegrationTamperedCacheRepaired(t *testing.T) {
	setTempDriverCache(t)
	stopDriver(t)

	target, err := driverFilePath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
	require.NoError(t, os.WriteFile(target, []byte("planted payload, not the WinDivert driver"), 0o644))

	h := openHandle(t, nil, FlagSendOnly)
	require.NoError(t, h.Close())

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	require.True(t, bytes.Equal(content, sysBytes), "cached driver was not repaired to the embedded asset")
}

func TestIntegrationDriverFileLockedWhileHeld(t *testing.T) {
	setTempDriverCache(t)

	sysPath, sysFile, err := openVerifiedDriver()
	require.NoError(t, err)
	defer sysFile.Close()

	writeErr := os.WriteFile(sysPath, []byte("overwrite attempt"), 0o644)
	require.Error(t, writeErr)
	require.True(t, errors.Is(writeErr, windows.ERROR_SHARING_VIOLATION),
		"expected sharing violation, got %v", writeErr)

	evil := sysPath + ".evil"
	require.NoError(t, os.WriteFile(evil, []byte("replacement attempt"), 0o644))
	defer os.Remove(evil)
	renameErr := os.Rename(evil, sysPath)
	require.Error(t, renameErr)
}
