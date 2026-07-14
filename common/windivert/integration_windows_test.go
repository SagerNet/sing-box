//go:build windows

package windivert

import (
	"bytes"
	"errors"
	"log"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sagernet/sing-box/internal/winmutex"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestMain(m *testing.M) {
	exitCode, err := winmutex.WithLock("SingBoxWinDivertIntegrationTests", 3*time.Minute, func() (int, error) {
		return m.Run(), nil
	})
	if err != nil {
		log.Print(E.Cause(err, "run in exclusive WinDivert integration test environment"))
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func openHandle(t *testing.T, filter *Filter, flags Flag) *Handle {
	t.Helper()
	h, err := Open(filter, LayerNetwork, 0, flags)
	require.NoError(t, err)
	return h
}

// A send-only handle installs+opens the driver but does not attach a
// receive filter, so it exercises the full driver-install path without
// diverting any live traffic on the host.
func TestIntegrationOpenSendOnly(t *testing.T) {
	h := openHandle(t, nil, FlagSendOnly)
	require.NoError(t, h.Close())
}

// Close is idempotent per the doc contract.
func TestIntegrationCloseTwice(t *testing.T) {
	h := openHandle(t, nil, FlagSendOnly)
	require.NoError(t, h.Close())
	require.NoError(t, h.Close())
}

// Recv must unblock when the handle is closed concurrently. Without this,
// the spoofer's run goroutine could deadlock on shutdown.
func TestIntegrationRecvAbortsOnClose(t *testing.T) {
	// A filter no live traffic will match, so Recv blocks indefinitely
	// until Close aborts the overlapped I/O.
	filter, err := OutboundTCP(
		netip.MustParseAddrPort("10.255.255.254:1"),
		netip.MustParseAddrPort("10.255.255.253:2"),
	)
	require.NoError(t, err)
	h := openHandle(t, filter, 0)

	errCh := make(chan error, 1)
	go func() {
		buf := make([]byte, MTUMax)
		_, _, recvErr := h.Recv(buf)
		errCh <- recvErr
	}()

	// Let Recv reach the blocking DeviceIoControl before Close races in.
	time.Sleep(200 * time.Millisecond)
	require.NoError(t, h.Close())

	select {
	case err := <-errCh:
		require.Error(t, err)
		require.True(t, errors.Is(err, windows.ERROR_OPERATION_ABORTED),
			"Recv should return ERROR_OPERATION_ABORTED, got %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("Recv did not unblock within 3s after Close")
	}
}

func cachedDriverPath(t *testing.T) string {
	t.Helper()
	base, err := os.UserCacheDir()
	require.NoError(t, err)
	return filepath.Join(base, "sing-box", "windivert", "v"+AssetVersion, driverSysName())
}

// The driver does not unload when the last handle closes: it stays running
// (and the memory manager keeps its backing image write-locked) until
// explicitly stopped, like `sc stop WinDivert`. The install-time
// DeleteService mark then removes the record once the last SCM handle
// closes.
func stopDriver(t *testing.T) {
	t.Helper()
	manager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	require.NoError(t, err)
	defer windows.CloseServiceHandle(manager)
	serviceNameW, err := windows.UTF16PtrFromString(driverServiceName)
	require.NoError(t, err)
	service, err := windows.OpenService(manager, serviceNameW, windows.SERVICE_STOP|windows.SERVICE_QUERY_STATUS)
	if err == nil {
		defer windows.CloseServiceHandle(service)
		var status windows.SERVICE_STATUS
		err = windows.ControlService(service, windows.SERVICE_CONTROL_STOP, &status)
		if err != nil &&
			!errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) &&
			!errors.Is(err, windows.ERROR_SERVICE_CANNOT_ACCEPT_CTRL) {
			require.NoError(t, err)
		}
		require.Eventually(t, func() bool {
			queryErr := windows.QueryServiceStatus(service, &status)
			return queryErr == nil && status.CurrentState == windows.SERVICE_STOPPED
		}, 60*time.Second, 200*time.Millisecond, "driver did not reach SERVICE_STOPPED")
	} else {
		require.True(t, errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST), "open driver service: %v", err)
	}
	// SCM can report SERVICE_STOPPED before the driver finishes deleting its
	// device object. Wait for the absence acquireDevice uses to trigger install.
	require.Eventually(t, func() bool {
		device, openErr := openDevice()
		if openErr == nil {
			_ = windows.CloseHandle(device)
			return false
		}
		return errors.Is(openErr, windows.ERROR_FILE_NOT_FOUND) ||
			errors.Is(openErr, windows.ERROR_PATH_NOT_FOUND) ||
			errors.Is(openErr, windows.ERROR_NO_SUCH_DEVICE)
	}, 60*time.Second, 200*time.Millisecond, "driver device remained openable after stop")
}

// The image lock on the cached .sys can outlive SERVICE_STOPPED by tens of
// seconds (observed on GitHub-hosted runners), and on current runner images
// it blocks renames as well as writes and deletes. Tests that need to tamper
// with the cache therefore redirect it to a directory the kernel has never
// loaded a driver from. Not t.TempDir: once StartService maps a .sys from
// the directory, the image lock makes the cleanup RemoveAll fail the test.
func setTempDriverCache(t *testing.T) {
	t.Helper()
	dir, err := os.MkdirTemp("", "sing-box-windivert-test-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("LocalAppData", dir)
}

// A foreign .sys planted in the user-writable cache must never reach
// StartService: the install path has to detect the mismatch against the
// embedded asset and repair the file before handing it to SCM.
func TestIntegrationTamperedCacheRepaired(t *testing.T) {
	setTempDriverCache(t)
	// The driver left running by earlier tests would satisfy Open without
	// touching the cache; stop it so the install path runs.
	stopDriver(t)

	target := cachedDriverPath(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
	require.NoError(t, os.WriteFile(target, []byte("planted payload, not the WinDivert driver"), 0o644))

	h := openHandle(t, nil, FlagSendOnly)
	require.NoError(t, h.Close())

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	require.True(t, bytes.Equal(content, sysBytes), "cached driver was not repaired to the embedded asset")
}

// The verified handle must lock the file against writers and renames until
// install completes; without this, the file could be swapped between
// verification and the kernel mapping it.
func TestIntegrationDriverFileLockedWhileHeld(t *testing.T) {
	// A fresh cache directory guarantees the failures asserted below can
	// only come from the handle extractVerified holds, not a kernel image
	// lock left by earlier tests.
	setTempDriverCache(t)

	sysPath, sysFile, err := extractVerified()
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

// Two concurrent Open calls must both succeed: the first wins the driver
// install race, the second reuses the already-running service.
func TestIntegrationConcurrentOpen(t *testing.T) {
	stopDriver(t)
	start := make(chan struct{})
	errCh := make(chan error, 2)
	handles := make(chan *Handle, 2)
	for range 2 {
		go func() {
			<-start
			h, err := Open(nil, LayerNetwork, 0, FlagSendOnly)
			handles <- h
			errCh <- err
		}()
	}
	close(start)
	for range 2 {
		err := <-errCh
		h := <-handles
		require.NoError(t, err)
		require.NoError(t, h.Close())
	}
}
