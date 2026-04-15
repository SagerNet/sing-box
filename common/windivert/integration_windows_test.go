//go:build windows

package windivert

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

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

// Two concurrent Open calls must both succeed: the first wins the driver
// install race, the second reuses the already-running service.
func TestIntegrationConcurrentOpen(t *testing.T) {
	errCh := make(chan error, 2)
	handles := make(chan *Handle, 2)
	for i := 0; i < 2; i++ {
		go func() {
			h, err := Open(nil, LayerNetwork, 0, FlagSendOnly)
			handles <- h
			errCh <- err
		}()
	}
	for i := 0; i < 2; i++ {
		err := <-errCh
		h := <-handles
		require.NoError(t, err)
		require.NoError(t, h.Close())
	}
}
