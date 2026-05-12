//go:build darwin && cgo

package usbip

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"

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
		logger:  newTestLogger(t),
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
	session := newDarwinServerDataSession(context.Background(), newTestLogger(t), serverConn, device)

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
	case <-session.Done():
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
	host := newDarwinExportHost(newTestLogger(t), nil, darwinServerOps{
		copyUSBHostDevices: func() ([]darwinUSBHostDeviceInfo, error) {
			t.Fatal("Reconcile should not run after cancel")
			return nil, nil
		},
	})
	server := &ServerService{
		ctx:          ctx,
		logger:       newTestLogger(t),
		host:         host,
		exports:      make(map[string]Export),
		busy:         make(map[string]bool),
		controlSubs:  make(map[uint64]*serverControlConn),
		controlState: make(map[string]DeviceInfoV2),
	}

	require.NoError(t, server.reconcileAndBroadcast(true))
}

func TestDarwinExportHostEventTriggersReconcile(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const busid = "mac-00000001"
	entry := standardTestDeviceEntry(busid)
	info := darwinTestDeviceInfo(1, entry)
	var (
		devices   []darwinUSBHostDeviceInfo
		fakeWatch *fakeDarwinUSBHostDeviceWatch
	)
	ops := darwinServerOps{
		copyUSBHostDevices: func() ([]darwinUSBHostDeviceInfo, error) {
			return devices, nil
		},
		openUSBHostDevice: func(registryID uint64, capture bool) (*darwinUSBHostDevice, error) {
			require.Equal(t, info.registryID, registryID)
			require.True(t, capture)
			return &darwinUSBHostDevice{info: info}, nil
		},
		watchUSBHostDevices: func(callback func()) (darwinUSBHostDeviceWatch, error) {
			fakeWatch = &fakeDarwinUSBHostDeviceWatch{callback: callback}
			return fakeWatch, nil
		},
	}
	host := newDarwinExportHost(newTestLogger(t), []option.USBIPDeviceMatch{{BusID: busid}}, ops)

	events, err := host.Events(ctx)
	require.NoError(t, err)
	require.NotNil(t, fakeWatch)

	devices = []darwinUSBHostDeviceInfo{info}
	fakeWatch.trigger()

	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Events channel signal")
	}

	snapshot, released, changed, err := host.Reconcile(ctx, func(string) bool { return false })
	require.NoError(t, err)
	require.True(t, changed)
	require.Empty(t, released)
	require.Contains(t, snapshot, busid)
}

func TestDarwinServerBuildDeviceStateIncludesBusyExports(t *testing.T) {
	t.Parallel()

	available := standardTestDeviceEntry("available")
	busy := standardTestDeviceEntry("busy")
	exports := map[string]Export{
		"available": &darwinExport{
			busid:      "available",
			registryID: 1,
			entry:      available,
		},
		"busy": &darwinExport{
			busid:      "busy",
			registryID: 2,
			entry:      busy,
		},
	}
	server := &ServerService{
		ctx:     context.Background(),
		logger:  newTestLogger(t),
		exports: exports,
		busy:    map[string]bool{"busy": true},
	}

	devices := deviceInfoV2Map(server.buildDeviceStateV2())
	require.Equal(t, deviceStateAvailable, devices["available"].State)
	require.Equal(t, deviceStateBusy, devices["busy"].State)
}

func TestDarwinExportHostReconcileMarksBusyMissingExportStale(t *testing.T) {
	t.Parallel()

	const busid = "mac-00000001"
	entry := standardTestDeviceEntry(busid)
	ops := darwinServerOps{
		copyUSBHostDevices: func() ([]darwinUSBHostDeviceInfo, error) {
			return nil, nil
		},
	}
	host := newDarwinExportHost(newTestLogger(t), []option.USBIPDeviceMatch{{BusID: busid}}, ops)
	host.exports[busid] = &darwinExport{
		busid:      busid,
		registryID: 1,
		device:     &darwinUSBHostDevice{},
		entry:      entry,
	}

	snapshot, released, changed, err := host.Reconcile(context.Background(), func(b string) bool {
		return b == busid
	})
	require.NoError(t, err)
	require.True(t, changed)
	require.Empty(t, released)
	require.NotContains(t, snapshot, busid)

	require.True(t, host.exports[busid].stale)

	releasedFinish, err := host.FinishImport(context.Background(), busid)
	require.NoError(t, err)
	require.True(t, releasedFinish)
	require.NotContains(t, host.exports, busid)
}

func TestDarwinExportHostReconcileCapturesReplacementAfterStaleRelease(t *testing.T) {
	t.Parallel()

	const busid = "mac-00000001"
	oldEntry := standardTestDeviceEntry(busid)
	replacementEntry := standardTestDeviceEntry(busid)
	replacementEntry.Info.DevNum = 2
	replacementInfo := darwinTestDeviceInfo(2, replacementEntry)
	var (
		opened int
		feed   = []darwinUSBHostDeviceInfo{replacementInfo}
	)
	ops := darwinServerOps{
		copyUSBHostDevices: func() ([]darwinUSBHostDeviceInfo, error) {
			return feed, nil
		},
		openUSBHostDevice: func(registryID uint64, capture bool) (*darwinUSBHostDevice, error) {
			require.Equal(t, replacementInfo.registryID, registryID)
			require.True(t, capture)
			opened++
			return &darwinUSBHostDevice{info: replacementInfo}, nil
		},
	}
	host := newDarwinExportHost(newTestLogger(t), []option.USBIPDeviceMatch{{BusID: busid}}, ops)
	host.exports[busid] = &darwinExport{
		busid:      busid,
		registryID: 1,
		device:     &darwinUSBHostDevice{},
		entry:      oldEntry,
	}

	snapshot, released, changed, err := host.Reconcile(context.Background(), func(b string) bool {
		return b == busid
	})
	require.NoError(t, err)
	require.True(t, changed)
	require.Empty(t, released)
	require.NotContains(t, snapshot, busid)
	require.True(t, host.exports[busid].stale)
	require.Zero(t, opened)

	releasedFinish, err := host.FinishImport(context.Background(), busid)
	require.NoError(t, err)
	require.True(t, releasedFinish)

	snapshot, released, changed, err = host.Reconcile(context.Background(), func(string) bool { return false })
	require.NoError(t, err)
	require.True(t, changed)
	require.Empty(t, released)
	require.Contains(t, snapshot, busid)
	require.Equal(t, 1, opened)
}

func TestDarwinServerRegisterControlConnQueuesSnapshotBeforeBroadcast(t *testing.T) {
	t.Parallel()

	server := &ServerService{
		ctx:          context.Background(),
		logger:       newTestLogger(t),
		exports:      make(map[string]Export),
		busy:         make(map[string]bool),
		controlSubs:  make(map[uint64]*serverControlConn),
		controlState: make(map[string]DeviceInfoV2),
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	sub, seq := server.registerControlConn(serverConn, controlCapabilities)
	require.Zero(t, seq)
	require.Contains(t, server.controlSubs, sub.id)

	added := standardTestDeviceEntry("added")
	server.exports["added"] = &darwinExport{busid: "added", registryID: 1, entry: added}
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
	host := newDarwinExportHost(log.NewNOPFactory().NewLogger("usbip"), nil, darwinServerOps{
		copyUSBHostDevices: func() ([]darwinUSBHostDeviceInfo, error) {
			return nil, nil
		},
	})
	host.exports[busid] = &darwinExport{
		busid:      busid,
		registryID: 1,
		entry:      entry,
		device:     &darwinUSBHostDevice{},
	}
	server := &ServerService{
		ctx:          ctx,
		logger:       log.NewNOPFactory().NewLogger("usbip"),
		host:         host,
		exports:      map[string]Export{busid: host.exports[busid]},
		busy:         make(map[string]bool),
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
	server.controlAccess.Lock()
	defer server.controlAccess.Unlock()
	return server.controlState[busid].State
}

func darwinTestDeviceInfo(registryID uint64, entry DeviceEntry) darwinUSBHostDeviceInfo {
	busid := entry.Info.BusIDString()
	return darwinUSBHostDeviceInfo{
		registryID: registryID,
		entry:      entry,
		key: DeviceKey{
			BusID:     busid,
			VendorID:  entry.Info.IDVendor,
			ProductID: entry.Info.IDProduct,
			Serial:    entrySerial(entry),
		},
	}
}

type fakeDarwinUSBHostDeviceWatch struct {
	access   sync.Mutex
	callback func()
	closed   bool
}

func (w *fakeDarwinUSBHostDeviceWatch) Close() {
	w.access.Lock()
	defer w.access.Unlock()
	w.closed = true
}

func (w *fakeDarwinUSBHostDeviceWatch) trigger() {
	w.callback()
}

func (w *fakeDarwinUSBHostDeviceWatch) isClosed() bool {
	w.access.Lock()
	defer w.access.Unlock()
	return w.closed
}
