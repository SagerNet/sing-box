//go:build windows && !with_external_windivert

package windivert

import (
	"errors"
	"log"
	"net/netip"
	"os"
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

func TestIntegrationOpenSendOnly(t *testing.T) {
	h := openHandle(t, nil, FlagSendOnly)
	require.NoError(t, h.Close())
}

func TestIntegrationCloseTwice(t *testing.T) {
	h := openHandle(t, nil, FlagSendOnly)
	require.NoError(t, h.Close())
	require.NoError(t, h.Close())
}

func TestIntegrationRecvAbortsOnClose(t *testing.T) {
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

// The driver does not unload when the last handle closes: it stays running
// until explicitly stopped, like `sc stop WinDivert`.
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
	// device object.
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
