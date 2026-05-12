//go:build linux || (darwin && cgo)

package usbip

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box/log"

	"github.com/stretchr/testify/require"
)

type fakeExport struct {
	busid       string
	leaseOK     bool
	leaseReason string
	leaseHook   func()
}

func (f *fakeExport) BusID() string { return f.busid }

func (f *fakeExport) Snapshot(ctx context.Context, busy bool) ExportSnapshot {
	state := deviceStateAvailable
	if busy {
		state = deviceStateBusy
	}
	var info DeviceInfoTruncated
	copy(info.BusID[:], f.busid)
	info.IDVendor = 0x1d6b
	info.IDProduct = 0x0002
	return ExportSnapshot{
		Entry:    DeviceEntry{Info: info, Serial: f.busid},
		Backend:  "fake",
		StableID: "fake:" + f.busid,
		State:    state,
	}
}

func (f *fakeExport) LeaseCheck(ctx context.Context) (bool, string) {
	if f.leaseHook != nil {
		f.leaseHook()
	}
	if f.leaseOK {
		return true, ""
	}
	if f.leaseReason == "" {
		return false, "unavailable"
	}
	return false, f.leaseReason
}

func (f *fakeExport) DeviceInfo(ctx context.Context) (DeviceInfoTruncated, error) {
	var info DeviceInfoTruncated
	copy(info.BusID[:], f.busid)
	return info, nil
}

func (f *fakeExport) NewServerDataSession(ctx context.Context, conn net.Conn) (DataSession, error) {
	return nil, nil
}

func newTestExportLedger(t testing.TB, now time.Time) *exportLedger {
	t.Helper()
	frozen := now
	return newExportLedger(log.NewNOPFactory().NewLogger("usbip"), importLeaseTTL, func() time.Time { return frozen })
}

func TestExportLedgerConsumeAcceptsCurrentGeneration(t *testing.T) {
	t.Parallel()

	now := time.Now()
	const busid = "1-1"
	ledger := newTestExportLedger(t, now)
	ledger.leases[busid] = serverImportLease{
		ID:          55,
		BusID:       busid,
		ClientNonce: 99,
		Generation:  0,
		Expires:     now.Add(importLeaseTTL),
	}

	require.True(t, ledger.ConsumeLease(ImportExtRequest{
		BusID:       busid,
		LeaseID:     55,
		ClientNonce: 99,
	}))
	require.Empty(t, ledger.leases)
}

func TestExportLedgerConsumeRejectsStaleGeneration(t *testing.T) {
	t.Parallel()

	now := time.Now()
	const busid = "1-1"
	ledger := newTestExportLedger(t, now)
	ledger.seq = 8
	ledger.leases[busid] = serverImportLease{
		ID:          55,
		BusID:       busid,
		ClientNonce: 99,
		Generation:  7,
		Expires:     now.Add(importLeaseTTL),
	}

	require.False(t, ledger.ConsumeLease(ImportExtRequest{
		BusID:       busid,
		LeaseID:     55,
		ClientNonce: 99,
	}))
	require.Empty(t, ledger.leases)
}

func TestExportLedgerConsumeRejectsExpiredLease(t *testing.T) {
	t.Parallel()

	now := time.Now()
	const busid = "1-1"
	ledger := newTestExportLedger(t, now)
	ledger.leases[busid] = serverImportLease{
		ID:          55,
		BusID:       busid,
		ClientNonce: 99,
		Generation:  0,
		Expires:     now.Add(-time.Second),
	}

	require.False(t, ledger.ConsumeLease(ImportExtRequest{
		BusID:       busid,
		LeaseID:     55,
		ClientNonce: 99,
	}))
	require.Empty(t, ledger.leases)
}

func TestExportLedgerConsumeRejectsMismatchedNonce(t *testing.T) {
	t.Parallel()

	now := time.Now()
	const busid = "1-1"
	ledger := newTestExportLedger(t, now)
	ledger.leases[busid] = serverImportLease{
		ID:          55,
		BusID:       busid,
		ClientNonce: 99,
		Generation:  0,
		Expires:     now.Add(importLeaseTTL),
	}

	require.False(t, ledger.ConsumeLease(ImportExtRequest{
		BusID:       busid,
		LeaseID:     55,
		ClientNonce: 100,
	}))
	require.Len(t, ledger.leases, 1)
}

func TestExportLedgerIssueRejectsWhenBusy(t *testing.T) {
	t.Parallel()

	now := time.Now()
	const busid = "1-1"
	ledger := newTestExportLedger(t, now)
	ledger.exports[busid] = &fakeExport{busid: busid, leaseOK: true}
	ledger.busy[busid] = true

	response := ledger.IssueLease(context.Background(), 1, controlLeaseRequest{BusID: busid, ClientNonce: 42})
	require.Equal(t, leaseErrorUnavailable, response.ErrorCode)
	require.Empty(t, ledger.leases)
}

func TestExportLedgerIssueRejectsWhenLeaseAlreadyActive(t *testing.T) {
	t.Parallel()

	now := time.Now()
	const busid = "1-1"
	ledger := newTestExportLedger(t, now)
	ledger.exports[busid] = &fakeExport{busid: busid, leaseOK: true}

	first := ledger.IssueLease(context.Background(), 1, controlLeaseRequest{BusID: busid, ClientNonce: 42})
	require.Empty(t, first.ErrorCode)

	second := ledger.IssueLease(context.Background(), 2, controlLeaseRequest{BusID: busid, ClientNonce: 43})
	require.Equal(t, leaseErrorBusy, second.ErrorCode)
	require.Len(t, ledger.leases, 1)
}

func TestExportLedgerIssueConcurrentlyAcceptsOne(t *testing.T) {
	t.Parallel()

	const busid = "1-1"
	const workers = 64
	ledger := newExportLedger(log.NewNOPFactory().NewLogger("usbip"), importLeaseTTL, time.Now)
	ledger.exports[busid] = &fakeExport{busid: busid, leaseOK: true}

	var (
		accepted atomic.Uint32
		busyHits atomic.Uint32
		wg       sync.WaitGroup
	)
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			response := ledger.IssueLease(context.Background(), uint64(i+1), controlLeaseRequest{
				BusID:       busid,
				ClientNonce: uint64(i + 1),
			})
			switch response.ErrorCode {
			case "":
				accepted.Add(1)
			case leaseErrorBusy:
				busyHits.Add(1)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	require.Equal(t, uint32(1), accepted.Load())
	require.Equal(t, uint32(workers-1), busyHits.Load())
	require.Len(t, ledger.leases, 1)
}

func TestExportLedgerIssueLeaseCheckIsAtomicWithInsert(t *testing.T) {
	t.Parallel()

	const busid = "1-1"
	ledger := newExportLedger(log.NewNOPFactory().NewLogger("usbip"), importLeaseTTL, time.Now)

	leaseCheckEntered := make(chan struct{}, 1)
	blockLeaseCheck := make(chan struct{})
	export := &fakeExport{busid: busid, leaseOK: true, leaseHook: func() {
		select {
		case leaseCheckEntered <- struct{}{}:
		default:
		}
		<-blockLeaseCheck
	}}
	ledger.exports[busid] = export

	go func() {
		ledger.IssueLease(context.Background(), 1, controlLeaseRequest{BusID: busid, ClientNonce: 1})
	}()
	<-leaseCheckEntered

	concurrent := make(chan controlLeaseResponse, 1)
	go func() {
		export2 := &fakeExport{busid: busid, leaseOK: true}
		ledger2 := newExportLedger(log.NewNOPFactory().NewLogger("usbip"), importLeaseTTL, time.Now)
		ledger2.exports[busid] = export2
		concurrent <- ledger2.IssueLease(context.Background(), 2, controlLeaseRequest{BusID: busid, ClientNonce: 2})
	}()

	select {
	case <-concurrent:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("independent ledger should not block on first lease check")
	}

	close(blockLeaseCheck)
}

func TestExportLedgerUnsubscribeRevokesOwnedLeases(t *testing.T) {
	t.Parallel()

	ledger := newExportLedger(log.NewNOPFactory().NewLogger("usbip"), importLeaseTTL, time.Now)
	for _, busid := range []string{"1-1", "1-2", "2-1"} {
		ledger.exports[busid] = &fakeExport{busid: busid, leaseOK: true}
	}

	pipeA, _ := net.Pipe()
	pipeB, _ := net.Pipe()
	subA, _ := ledger.Subscribe(context.Background(), pipeA, controlCapabilities)
	subB, _ := ledger.Subscribe(context.Background(), pipeB, controlCapabilities)
	defer pipeA.Close()
	defer pipeB.Close()

	for _, spec := range []struct {
		sub   *exportSubscriber
		busid string
	}{
		{subA, "1-1"},
		{subA, "1-2"},
		{subB, "2-1"},
	} {
		response := ledger.IssueLease(context.Background(), spec.sub.id, controlLeaseRequest{
			BusID:       spec.busid,
			ClientNonce: spec.sub.id,
		})
		require.Empty(t, response.ErrorCode, spec.busid)
	}
	require.Len(t, ledger.leases, 3)

	ledger.Unsubscribe(subA)
	require.Len(t, ledger.leases, 1)
	_, found := ledger.leases["2-1"]
	require.True(t, found)
}

func TestExportLedgerBroadcastEmitsOnlyOnDelta(t *testing.T) {
	t.Parallel()

	const busid = "1-1"
	ledger := newExportLedger(log.NewNOPFactory().NewLogger("usbip"), importLeaseTTL, time.Now)
	ledger.exports[busid] = &fakeExport{busid: busid, leaseOK: true}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	sub, _ := ledger.Subscribe(context.Background(), serverConn, controlCapabilities)

	// drain the initial snapshot
	<-sub.send

	// first broadcast: new export shows up as Added
	require.True(t, ledger.BroadcastIfChanged(context.Background()))
	delta := <-sub.send
	require.Equal(t, controlFrameDeviceDelta, delta.Frame.Type)

	// second broadcast against identical state: must NOT emit
	require.False(t, ledger.BroadcastIfChanged(context.Background()))
	select {
	case msg := <-sub.send:
		t.Fatalf("unexpected broadcast frame: %v", msg.Frame.Type)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestExportLedgerSubscriberLagClosesConn(t *testing.T) {
	t.Parallel()

	ledger := newExportLedger(log.NewNOPFactory().NewLogger("usbip"), importLeaseTTL, time.Now)
	serverConn, clientConn := net.Pipe()
	sub, _ := ledger.Subscribe(context.Background(), serverConn, controlRequiredCapabilities)

	// Fill the buffer. Subscribe enqueued nothing (no extension capabilities)
	for i := 0; i < controlSubscriberSendBuffer; i++ {
		ledger.enqueueFrame(sub, controlFrame{Type: controlFrameChanged, Version: controlProtocolVersion, Sequence: uint64(i + 1)})
	}

	// One more should trigger a close.
	ledger.enqueueFrame(sub, controlFrame{Type: controlFrameChanged, Version: controlProtocolVersion, Sequence: 999})
	require.Eventually(t, func() bool {
		_ = clientConn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
		_, err := clientConn.Read(make([]byte, 1))
		return err != nil
	}, time.Second, 10*time.Millisecond)
	_ = clientConn.Close()
}

func TestExportLedgerTryReserveSerializes(t *testing.T) {
	t.Parallel()

	const busid = "1-1"
	const workers = 32
	ledger := newExportLedger(log.NewNOPFactory().NewLogger("usbip"), importLeaseTTL, time.Now)
	ledger.exports[busid] = &fakeExport{busid: busid, leaseOK: true}

	var (
		accepted atomic.Uint32
		busyHits atomic.Uint32
		wg       sync.WaitGroup
	)
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, ok, reason := ledger.TryReserveForImport(context.Background(), busid)
			if ok {
				accepted.Add(1)
				return
			}
			if reason == deviceStateBusy {
				busyHits.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	require.Equal(t, uint32(1), accepted.Load())
	require.Equal(t, uint32(workers-1), busyHits.Load())
	require.True(t, ledger.busy[busid])
}

func TestExportLedgerApplyHostSnapshotClearsReleasedBusy(t *testing.T) {
	t.Parallel()

	const busid = "1-1"
	ledger := newExportLedger(log.NewNOPFactory().NewLogger("usbip"), importLeaseTTL, time.Now)
	ledger.exports[busid] = &fakeExport{busid: busid, leaseOK: true}
	ledger.busy[busid] = true

	ledger.ApplyHostSnapshot(map[string]Export{}, []string{busid})

	require.False(t, ledger.busy[busid])
	require.NotContains(t, ledger.exports, busid)
}

func TestExportLedgerIssueConcurrentWithBroadcastDoesNotDeadlock(t *testing.T) {
	t.Parallel()

	const busid = "1-1"
	ledger := newExportLedger(log.NewNOPFactory().NewLogger("usbip"), importLeaseTTL, time.Now)
	ledger.exports[busid] = &fakeExport{busid: busid, leaseOK: true}

	done := make(chan struct{}, 2)
	go func() {
		for i := 0; i < 1000; i++ {
			ledger.IssueLease(context.Background(), 1, controlLeaseRequest{BusID: busid, ClientNonce: uint64(i)})
			ledger.slow.Lock()
			delete(ledger.leases, busid)
			ledger.slow.Unlock()
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 1000; i++ {
			ledger.BroadcastIfChanged(context.Background())
		}
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("issue+broadcast deadlocked")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("second goroutine deadlocked")
	}
}
