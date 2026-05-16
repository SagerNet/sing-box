//go:build darwin && cgo

package usbip

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDarwinStaleExportBroadcastsUnavailableUpdate(t *testing.T) {
	t.Parallel()

	ledger := newExportLedger(nil, time.Second, func() time.Time { return time.Unix(0, 0) })
	entry := darwinFakeDeviceEntry()
	export := &darwinExport{
		busid:      entry.Info.BusIDString(),
		registryID: 0x1234,
		entry:      entry,
	}

	ledger.ApplyHostSnapshot(map[string]Export{export.busid: export}, nil)
	ledger.SeedBroadcastState()

	sub, _ := ledger.Subscribe(nil, controlCapabilities)
	select {
	case <-sub.send:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for initial snapshot")
	}

	export.stale = true
	export.pendingRegistryID = 0x5678

	if !ledger.BroadcastIfChanged() {
		t.Fatal("expected stale darwin export to broadcast an update")
	}

	select {
	case message := <-sub.send:
		require.Equal(t, controlFrameDeviceDelta, message.Frame.Type)

		var delta controlDeviceDelta
		require.NoError(t, unmarshalControlPayload(message.Payload, &delta))
		require.Empty(t, delta.Removed)
		require.Len(t, delta.Updated, 1)
		require.Equal(t, export.busid, delta.Updated[0].BusID)
		require.Equal(t, deviceStateUnavailable, delta.Updated[0].State)
		require.Equal(t, "device replaced", delta.Updated[0].StatusReason)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for unavailable update")
	}
}
