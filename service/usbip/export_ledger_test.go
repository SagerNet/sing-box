//go:build linux || (darwin && cgo)

package usbip

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestSubscribeRetriesSnapshotWhenSequenceAdvances(t *testing.T) {
	ctx := context.Background()
	ledger := newExportLedger(nil, time.Second, func() time.Time { return time.Unix(0, 0) })
	oldExport := &testExport{busid: "1-1", vendorID: 0x1111, productID: 0x0001}
	newExport := &testExport{busid: "2-1", vendorID: 0x2222, productID: 0x0002}

	ledger.ApplyHostSnapshot(map[string]Export{oldExport.busid: oldExport}, nil)
	ledger.SeedBroadcastState(ctx)

	oldExport.onSnapshot = func() {
		ledger.ApplyHostSnapshot(map[string]Export{newExport.busid: newExport}, nil)
		if !ledger.BroadcastIfChanged(ctx) {
			t.Fatal("expected broadcast after replacing export")
		}
	}

	sub, sequence := ledger.Subscribe(ctx, nil, controlCapabilities)
	if sequence != 1 {
		t.Fatalf("expected subscription sequence 1, got %d", sequence)
	}

	select {
	case message := <-sub.send:
		if message.Frame.Type != controlFrameDeviceSnapshot {
			t.Fatalf("expected device snapshot frame, got %d", message.Frame.Type)
		}
		if message.Frame.Sequence != sequence {
			t.Fatalf("expected frame sequence %d, got %d", sequence, message.Frame.Sequence)
		}
		var snapshot controlDeviceSnapshot
		if err := unmarshalControlPayload(message.Payload, &snapshot); err != nil {
			t.Fatal(err)
		}
		if snapshot.Sequence != sequence {
			t.Fatalf("expected payload sequence %d, got %d", sequence, snapshot.Sequence)
		}
		if len(snapshot.Devices) != 1 || snapshot.Devices[0].BusID != newExport.busid {
			t.Fatalf("expected fresh snapshot for %s, got %#v", newExport.busid, snapshot.Devices)
		}
	default:
		t.Fatal("expected queued device snapshot")
	}
}

type testExport struct {
	busid     string
	vendorID  uint16
	productID uint16

	onSnapshot func()
}

func (e *testExport) BusID() string {
	return e.busid
}

func (e *testExport) Snapshot(ctx context.Context, busy bool) ExportSnapshot {
	onSnapshot := e.onSnapshot
	e.onSnapshot = nil
	if onSnapshot != nil {
		onSnapshot()
	}
	return ExportSnapshot{
		Entry: DeviceEntry{
			Info: e.deviceInfo(),
		},
		State: deviceStateAvailable,
	}
}

func (e *testExport) LeaseCheck(ctx context.Context) (bool, string) {
	return true, ""
}

func (e *testExport) DeviceInfo(ctx context.Context) (DeviceInfoTruncated, error) {
	return e.deviceInfo(), nil
}

func (e *testExport) NewServerDataSession(ctx context.Context, conn net.Conn) (DataSession, error) {
	return nil, nil
}

func (e *testExport) deviceInfo() DeviceInfoTruncated {
	var info DeviceInfoTruncated
	copy(info.BusID[:], e.busid)
	info.IDVendor = e.vendorID
	info.IDProduct = e.productID
	info.Speed = 2
	return info
}
