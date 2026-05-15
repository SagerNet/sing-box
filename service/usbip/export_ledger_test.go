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

func TestConsumeLeaseAndReserveRejectsIdentityReplacement(t *testing.T) {
	ctx := context.Background()
	ledger := newExportLedger(nil, time.Second, func() time.Time { return time.Unix(0, 0) })
	original := &testExport{busid: "1-1", vendorID: 0x1111, productID: 0x0001, identity: "linux:original"}
	replacement := &testExport{busid: "1-1", vendorID: 0x1111, productID: 0x0001, identity: "linux:replacement"}

	ledger.ApplyHostSnapshot(map[string]Export{original.busid: original}, nil)
	lease := ledger.IssueLease(ctx, 1, controlLeaseRequest{BusID: original.busid, ClientNonce: 7})
	ledger.ApplyHostSnapshot(map[string]Export{replacement.busid: replacement}, nil)

	_, ok, reason := ledger.ConsumeLeaseAndReserve(ctx, ImportExtRequest{
		BusID:       original.busid,
		LeaseID:     lease.LeaseID,
		ClientNonce: lease.ClientNonce,
	})
	if ok {
		t.Fatal("expected replaced export lease to be rejected")
	}
	if reason != "lease stale" {
		t.Fatalf("expected lease stale, got %q", reason)
	}
}

func TestConsumeLeaseAndReserveRejectsUnavailableExport(t *testing.T) {
	ctx := context.Background()
	ledger := newExportLedger(nil, time.Second, func() time.Time { return time.Unix(0, 0) })
	available := true
	exp := &testExport{
		busid:     "1-1",
		vendorID:  0x1111,
		productID: 0x0001,
		leaseCheck: func(context.Context) (bool, string) {
			if available {
				return true, ""
			}
			return false, "capture released"
		},
	}

	ledger.ApplyHostSnapshot(map[string]Export{exp.busid: exp}, nil)
	lease := ledger.IssueLease(ctx, 1, controlLeaseRequest{BusID: exp.busid, ClientNonce: 9})
	available = false

	_, ok, reason := ledger.ConsumeLeaseAndReserve(ctx, ImportExtRequest{
		BusID:       exp.busid,
		LeaseID:     lease.LeaseID,
		ClientNonce: lease.ClientNonce,
	})
	if ok {
		t.Fatal("expected unavailable export lease to be rejected")
	}
	if reason != "capture released" {
		t.Fatalf("expected capture released, got %q", reason)
	}

	_, ok, reason = ledger.ConsumeLeaseAndReserve(ctx, ImportExtRequest{
		BusID:       exp.busid,
		LeaseID:     lease.LeaseID,
		ClientNonce: lease.ClientNonce,
	})
	if ok {
		t.Fatal("expected consumed lease to stay unavailable on retry")
	}
	if reason != "lease not found" {
		t.Fatalf("expected consumed lease to disappear, got %q", reason)
	}
}

func TestConsumeLeaseAndReserveMarksBusyOnSuccess(t *testing.T) {
	ctx := context.Background()
	ledger := newExportLedger(nil, time.Second, func() time.Time { return time.Unix(0, 0) })
	exp := &testExport{busid: "1-1", vendorID: 0x1111, productID: 0x0001}

	ledger.ApplyHostSnapshot(map[string]Export{exp.busid: exp}, nil)
	lease := ledger.IssueLease(ctx, 1, controlLeaseRequest{BusID: exp.busid, ClientNonce: 11})

	reserved, ok, reason := ledger.ConsumeLeaseAndReserve(ctx, ImportExtRequest{
		BusID:       exp.busid,
		LeaseID:     lease.LeaseID,
		ClientNonce: lease.ClientNonce,
	})
	if !ok {
		t.Fatalf("expected lease reservation success, got %q", reason)
	}
	if reserved != exp {
		t.Fatal("expected to reserve the original export instance")
	}
	if !ledger.IsBusy(exp.busid) {
		t.Fatal("expected successful lease reservation to mark busid busy")
	}
}

type testExport struct {
	busid     string
	vendorID  uint16
	productID uint16

	identity   ExportLeaseIdentity
	leaseCheck func(context.Context) (bool, string)
	onSnapshot func()
}

func (e *testExport) BusID() string {
	return e.busid
}

func (e *testExport) LeaseIdentity() ExportLeaseIdentity {
	if e.identity != "" {
		return e.identity
	}
	return ExportLeaseIdentity(e.busid)
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
	if e.leaseCheck != nil {
		return e.leaseCheck(ctx)
	}
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
