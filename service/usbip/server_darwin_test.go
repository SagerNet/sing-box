//go:build darwin && cgo

package usbip

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/stretchr/testify/require"
)

func TestDarwinServerPendingSubmitUnlinkState(t *testing.T) {
	t.Parallel()

	session := &darwinServerDataSession{
		pending: make(map[uint32]darwinServerPendingSubmit),
	}

	const endpoint uint8 = 0x81
	session.trackSubmit(7, endpoint)

	unlinkedEndpoint, active := session.markSubmitUnlinked(7)
	require.True(t, active)
	require.Equal(t, endpoint, unlinkedEndpoint)
	require.False(t, session.finishSubmit(7))

	session.trackSubmit(8, endpoint)
	require.True(t, session.finishSubmit(8))

	_, active = session.markSubmitUnlinked(8)
	require.False(t, active)
}

func TestDarwinServerReconcileAndBroadcastSkipsAfterCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := &ServerService{
		ctx:          ctx,
		logger:       newTestLogger(),
		exports:      make(map[string]serverExport),
		controlSubs:  make(map[uint64]*serverControlConn),
		controlState: make(map[string]DeviceInfoV2),
	}

	require.NoError(t, server.reconcileAndBroadcast(true))
}

func TestDarwinServerBuildDeviceStateIncludesBusyExports(t *testing.T) {
	t.Parallel()

	available := standardTestDeviceEntry("available")
	busy := standardTestDeviceEntry("busy")
	server := &ServerService{
		exports: map[string]serverExport{
			"available": {
				busid:      "available",
				registryID: 1,
				entry:      available,
			},
			"busy": {
				busid:      "busy",
				registryID: 2,
				entry:      busy,
				busy:       true,
			},
		},
	}

	devices := deviceInfoV2Map(server.buildDeviceStateV2())
	require.Equal(t, deviceStateAvailable, devices["available"].State)
	require.Equal(t, deviceStateBusy, devices["busy"].State)
}

func TestDarwinServerImportBroadcastsBusyState(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const busid = "1-1"
	entry := standardTestDeviceEntry(busid)
	server := &ServerService{
		ctx:          ctx,
		logger:       log.NewNOPFactory().NewLogger("usbip"),
		exports:      map[string]serverExport{busid: {busid: busid, registryID: 1, entry: entry}},
		controlSubs:  make(map[uint64]*serverControlConn),
		controlState: make(map[string]DeviceInfoV2),
	}

	serverConn, clientConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		server.handleImportBusID(serverConn, busid, false)
	}()

	header, err := ReadOpHeader(clientConn)
	require.NoError(t, err)
	require.Equal(t, OpRepImport, header.Code)
	require.Equal(t, OpStatusOK, header.Status)
	_, err = ReadOpRepImportBody(clientConn)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return darwinServerControlState(server, busid) == deviceStateBusy
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, clientConn.Close())
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Darwin import session shutdown")
	}

	require.Eventually(t, func() bool {
		return darwinServerControlState(server, busid) == deviceStateAvailable
	}, time.Second, 10*time.Millisecond)
}

func darwinServerControlState(server *ServerService, busid string) string {
	server.controlMu.Lock()
	defer server.controlMu.Unlock()
	return server.controlState[busid].State
}
