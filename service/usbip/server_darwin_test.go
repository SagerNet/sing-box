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

func TestDarwinServerAbortPendingSubmitsMarksAndAbortsEndpoints(t *testing.T) {
	t.Parallel()

	device := &fakeDarwinServerDataDevice{}
	session := &darwinServerDataSession{
		logger:  newTestLogger(),
		device:  device,
		pending: make(map[uint32]darwinServerPendingSubmit),
	}

	session.trackSubmit(7, 0x81)
	session.trackSubmit(8, 0x81)
	session.trackSubmit(9, 0x02)
	session.abortPendingSubmits()

	require.Equal(t, []uint8{0x02, 0x81}, device.aborted)
	require.False(t, session.finishSubmit(7))
	require.False(t, session.finishSubmit(8))
	require.False(t, session.finishSubmit(9))
}

func TestDarwinServerServeAbortsPendingSubmitOnClose(t *testing.T) {
	t.Parallel()

	serverConn, clientConn := net.Pipe()
	device := &fakeDarwinServerDataDevice{
		ioStarted:   make(chan struct{}),
		abortNotify: make(chan struct{}),
	}
	session := newDarwinServerDataSession(context.Background(), newTestLogger(), serverConn, device)
	done := make(chan error, 1)
	go func() {
		done <- session.serve()
	}()

	require.NoError(t, WriteSubmitCommand(clientConn, SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			SeqNum:    1,
			Direction: USBIPDirIn,
			Endpoint:  1,
		},
		TransferBufferLength: 8,
	}))
	select {
	case <-device.ioStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pending Darwin IO")
	}

	require.NoError(t, clientConn.Close())
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Darwin session shutdown")
	}
	require.Equal(t, []uint8{0x81}, device.aborted)
}

type fakeDarwinServerDataDevice struct {
	ioStarted   chan struct{}
	abortNotify chan struct{}
	aborted     []uint8
}

func (d *fakeDarwinServerDataDevice) control(setup [8]byte, buffer []byte) (int32, int32, []byte, error) {
	return 0, 0, buffer, nil
}

func (d *fakeDarwinServerDataDevice) io(endpoint uint8, buffer []byte) (int32, int32, []byte, error) {
	if d.ioStarted != nil {
		close(d.ioStarted)
	}
	if d.abortNotify != nil {
		<-d.abortNotify
	}
	return usbipStatusECONNRESET, 0, buffer, nil
}

func (d *fakeDarwinServerDataDevice) iso(endpoint uint8, buffer []byte, startFrame int32, packets []IsoPacketDescriptor) (int32, int32, []byte, []IsoPacketDescriptor, error) {
	return 0, 0, buffer, packets, nil
}

func (d *fakeDarwinServerDataDevice) abortEndpoint(endpoint uint8) error {
	d.aborted = append(d.aborted, endpoint)
	if d.abortNotify != nil {
		close(d.abortNotify)
	}
	return nil
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

func TestDarwinServerRegisterControlConnQueuesSnapshotBeforeBroadcast(t *testing.T) {
	t.Parallel()

	server := &ServerService{
		logger:       newTestLogger(),
		exports:      make(map[string]serverExport),
		controlSubs:  make(map[uint64]*serverControlConn),
		controlState: make(map[string]DeviceInfoV2),
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	sub, seq := server.registerControlConn(serverConn, controlCapabilities)
	require.Zero(t, seq)
	require.Contains(t, server.controlSubs, sub.id)

	server.broadcastChanged()

	first := <-sub.send
	require.Equal(t, controlFrameDeviceSnapshot, first.Frame.Type)
	require.Zero(t, first.Frame.Sequence)

	second := <-sub.send
	require.Equal(t, controlFrameDeviceDelta, second.Frame.Type)
	require.Equal(t, uint64(1), second.Frame.Sequence)
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
