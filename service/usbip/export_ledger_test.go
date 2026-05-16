//go:build linux || (darwin && cgo)

package usbip

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestSubscribeRetriesSnapshotWhenSequenceAdvances(t *testing.T) {
	ledger := newExportLedger(nil, time.Second, func() time.Time { return time.Unix(0, 0) })
	oldExport := &testExport{busid: "1-1", vendorID: 0x1111, productID: 0x0001}
	newExport := &testExport{busid: "2-1", vendorID: 0x2222, productID: 0x0002}

	ledger.ApplyHostSnapshot(map[string]Export{oldExport.busid: oldExport}, nil)
	ledger.SeedBroadcastState()

	oldExport.onSnapshot = func() {
		ledger.ApplyHostSnapshot(map[string]Export{newExport.busid: newExport}, nil)
		if !ledger.BroadcastIfChanged() {
			t.Fatal("expected broadcast after replacing export")
		}
	}

	sub, sequence := ledger.Subscribe(nil, controlCapabilities)
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
		err := unmarshalControlPayload(message.Payload, &snapshot)
		if err != nil {
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
	ledger := newExportLedger(nil, time.Second, func() time.Time { return time.Unix(0, 0) })
	original := &testExport{busid: "1-1", vendorID: 0x1111, productID: 0x0001, identity: "linux:original"}
	replacement := &testExport{busid: "1-1", vendorID: 0x1111, productID: 0x0001, identity: "linux:replacement"}

	ledger.ApplyHostSnapshot(map[string]Export{original.busid: original}, nil)
	lease := ledger.IssueLease(1, controlLeaseRequest{BusID: original.busid, ClientNonce: 7})
	ledger.ApplyHostSnapshot(map[string]Export{replacement.busid: replacement}, nil)

	_, ok, reason := ledger.ConsumeLeaseAndReserve(ImportExtRequest{
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
	ledger := newExportLedger(nil, time.Second, func() time.Time { return time.Unix(0, 0) })
	available := true
	exp := &testExport{
		busid:     "1-1",
		vendorID:  0x1111,
		productID: 0x0001,
		leaseCheck: func() (bool, string) {
			if available {
				return true, ""
			}
			return false, "capture released"
		},
	}

	ledger.ApplyHostSnapshot(map[string]Export{exp.busid: exp}, nil)
	lease := ledger.IssueLease(1, controlLeaseRequest{BusID: exp.busid, ClientNonce: 9})
	available = false

	_, ok, reason := ledger.ConsumeLeaseAndReserve(ImportExtRequest{
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

	_, ok, reason = ledger.ConsumeLeaseAndReserve(ImportExtRequest{
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

func TestUnsubscribeBroadcastsLeaseRelease(t *testing.T) {
	ledger := newExportLedger(nil, time.Second, func() time.Time { return time.Unix(0, 0) })
	exp := &testExport{busid: "1-1", vendorID: 0x1111, productID: 0x0001}

	ledger.ApplyHostSnapshot(map[string]Export{exp.busid: exp}, nil)
	ledger.SeedBroadcastState()

	holder, _ := ledger.Subscribe(nil, controlCapabilities)
	drainSubscriber(holder)

	response := ledger.IssueLease(holder.id, controlLeaseRequest{BusID: exp.busid, ClientNonce: 7})
	if response.ErrorCode != "" {
		t.Fatalf("IssueLease failed: %s", response.ErrorMessage)
	}
	// Pretend a topology event flushed the busy state into the broadcast
	// baseline, matching the real-world scenario where hotplug churn keeps
	// l.state roughly in sync with reality. Without this the diff machinery
	// in BroadcastIfChanged has no busy→available transition to emit.
	ledger.BroadcastIfChanged()
	drainSubscriber(holder)

	observer, _ := ledger.Subscribe(nil, controlCapabilities)
	drainSubscriber(observer)

	ledger.Unsubscribe(holder)

	select {
	case msg := <-observer.send:
		if msg.Frame.Type != controlFrameDeviceDelta {
			t.Fatalf("expected device delta after lease release, got frame type %d", msg.Frame.Type)
		}
		var delta controlDeviceDelta
		err := unmarshalControlPayload(msg.Payload, &delta)
		if err != nil {
			t.Fatal(err)
		}
		if len(delta.Updated) != 1 || delta.Updated[0].State != deviceStateAvailable {
			t.Fatalf("expected one Updated entry flipped to available, got %#v", delta)
		}
	default:
		t.Fatal("expected observer to receive a delta after holder's lease was released")
	}
}

func drainSubscriber(sub *exportSubscriber) {
	for {
		select {
		case <-sub.send:
		default:
			return
		}
	}
}

func TestConsumeLeaseAndReserveMarksBusyOnSuccess(t *testing.T) {
	ledger := newExportLedger(nil, time.Second, func() time.Time { return time.Unix(0, 0) })
	exp := &testExport{busid: "1-1", vendorID: 0x1111, productID: 0x0001}

	ledger.ApplyHostSnapshot(map[string]Export{exp.busid: exp}, nil)
	lease := ledger.IssueLease(1, controlLeaseRequest{BusID: exp.busid, ClientNonce: 11})

	reserved, ok, reason := ledger.ConsumeLeaseAndReserve(ImportExtRequest{
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
	if !ledger.IsReserved(exp.busid) {
		t.Fatal("expected successful lease reservation to mark busid reserved")
	}
}

type testExport struct {
	busid     string
	vendorID  uint16
	productID uint16

	identity   ExportLeaseIdentity
	leaseCheck func() (bool, string)
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

func (e *testExport) Snapshot(busy bool) ExportSnapshot {
	onSnapshot := e.onSnapshot
	e.onSnapshot = nil
	if onSnapshot != nil {
		onSnapshot()
	}
	state := deviceStateAvailable
	if busy {
		state = deviceStateBusy
	}
	return ExportSnapshot{
		Entry: DeviceEntry{
			Info: e.deviceInfo(),
		},
		State: state,
	}
}

func (e *testExport) LeaseCheck() (bool, string) {
	if e.leaseCheck != nil {
		return e.leaseCheck()
	}
	return true, ""
}

func (e *testExport) DeviceInfo() (DeviceInfoTruncated, error) {
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
