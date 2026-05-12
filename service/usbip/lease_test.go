//go:build linux || (darwin && cgo)

package usbip

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newLeaseManagerWithLeases(now time.Time, leases map[string]serverImportLease) *LeaseManager {
	manager := NewLeaseManager(importLeaseTTL, func() time.Time { return now })
	for busid, lease := range leases {
		manager.leases[busid] = lease
	}
	return manager
}

func TestLeaseManagerConsumeAcceptsCurrentGeneration(t *testing.T) {
	t.Parallel()

	now := time.Now()
	const busid = "1-1"
	manager := newLeaseManagerWithLeases(now, map[string]serverImportLease{
		busid: {
			ID:          55,
			BusID:       busid,
			ClientNonce: 99,
			Generation:  7,
			Expires:     now.Add(importLeaseTTL),
		},
	})

	ok := manager.Consume(7, ImportExtRequest{
		BusID:       busid,
		LeaseID:     55,
		ClientNonce: 99,
	})
	require.True(t, ok)
	require.Empty(t, manager.leases)
}

func TestLeaseManagerConsumeRejectsStaleGeneration(t *testing.T) {
	t.Parallel()

	now := time.Now()
	const busid = "1-1"
	manager := newLeaseManagerWithLeases(now, map[string]serverImportLease{
		busid: {
			ID:          55,
			BusID:       busid,
			ClientNonce: 99,
			Generation:  7,
			Expires:     now.Add(importLeaseTTL),
		},
	})

	ok := manager.Consume(8, ImportExtRequest{
		BusID:       busid,
		LeaseID:     55,
		ClientNonce: 99,
	})
	require.False(t, ok)
	require.Empty(t, manager.leases)
}

func TestLeaseManagerConsumeRejectsExpiredLease(t *testing.T) {
	t.Parallel()

	now := time.Now()
	const busid = "1-1"
	manager := newLeaseManagerWithLeases(now, map[string]serverImportLease{
		busid: {
			ID:          55,
			BusID:       busid,
			ClientNonce: 99,
			Generation:  7,
			Expires:     now.Add(-time.Second),
		},
	})

	ok := manager.Consume(7, ImportExtRequest{
		BusID:       busid,
		LeaseID:     55,
		ClientNonce: 99,
	})
	require.False(t, ok)
	require.Empty(t, manager.leases)
}

func TestLeaseManagerConsumeRejectsMismatchedNonce(t *testing.T) {
	t.Parallel()

	now := time.Now()
	const busid = "1-1"
	manager := newLeaseManagerWithLeases(now, map[string]serverImportLease{
		busid: {
			ID:          55,
			BusID:       busid,
			ClientNonce: 99,
			Generation:  7,
			Expires:     now.Add(importLeaseTTL),
		},
	})

	ok := manager.Consume(7, ImportExtRequest{
		BusID:       busid,
		LeaseID:     55,
		ClientNonce: 100,
	})
	require.False(t, ok)
	// Mismatched nonce keeps the lease in place for the legitimate client
	// to redeem; consume-on-read fires only on ID/nonce match.
	require.Len(t, manager.leases, 1)
}

func TestLeaseManagerIssueRejectsWhenAvailableReturnsFalse(t *testing.T) {
	t.Parallel()

	manager := NewLeaseManager(importLeaseTTL, time.Now)
	response := manager.Issue(1, 1, controlLeaseRequest{BusID: "1-1", ClientNonce: 42}, func() (bool, string) {
		return false, "device busy"
	})
	require.Equal(t, leaseErrorUnavailable, response.ErrorCode)
	require.Equal(t, "device busy", response.ErrorMessage)
	require.Empty(t, manager.leases)
}

func TestLeaseManagerIssueRejectsWhenLeaseAlreadyActive(t *testing.T) {
	t.Parallel()

	manager := NewLeaseManager(importLeaseTTL, time.Now)
	first := manager.Issue(1, 1, controlLeaseRequest{BusID: "1-1", ClientNonce: 42}, func() (bool, string) {
		return true, ""
	})
	require.Empty(t, first.ErrorCode)

	second := manager.Issue(2, 1, controlLeaseRequest{BusID: "1-1", ClientNonce: 43}, func() (bool, string) {
		return true, ""
	})
	require.Equal(t, leaseErrorBusy, second.ErrorCode)
	require.Len(t, manager.leases, 1)
}

func TestLeaseManagerIssueConcurrentlyAcceptsOne(t *testing.T) {
	t.Parallel()

	manager := NewLeaseManager(importLeaseTTL, time.Now)
	const workers = 64
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
			response := manager.Issue(uint64(i+1), 1, controlLeaseRequest{
				BusID:       "1-1",
				ClientNonce: uint64(i + 1),
			}, func() (bool, string) {
				return true, ""
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
	require.Len(t, manager.leases, 1)
}

func TestLeaseManagerIssueAvailabilityCheckIsAtomicWithInsert(t *testing.T) {
	t.Parallel()

	manager := NewLeaseManager(importLeaseTTL, time.Now)
	checkRan := make(chan struct{}, 1)
	blockCheck := make(chan struct{})

	go func() {
		manager.Issue(1, 1, controlLeaseRequest{BusID: "1-1", ClientNonce: 1}, func() (bool, string) {
			checkRan <- struct{}{}
			<-blockCheck
			return true, ""
		})
	}()

	<-checkRan
	// While the first Issue's availability check is blocked under the
	// manager lock, a second Issue must block on the same lock instead
	// of slipping in between check and insert.
	concurrent := make(chan controlLeaseResponse, 1)
	go func() {
		concurrent <- manager.Issue(2, 1, controlLeaseRequest{BusID: "1-1", ClientNonce: 2}, func() (bool, string) {
			return true, ""
		})
	}()

	select {
	case <-concurrent:
		t.Fatal("second Issue completed while first held the lock")
	case <-time.After(50 * time.Millisecond):
	}

	close(blockCheck)
	response := <-concurrent
	require.Equal(t, leaseErrorBusy, response.ErrorCode)
}

func TestLeaseManagerRevokeSubscriberRemovesOwnedLeases(t *testing.T) {
	t.Parallel()

	manager := NewLeaseManager(importLeaseTTL, time.Now)
	for _, spec := range []struct {
		subID uint64
		busid string
	}{
		{1, "1-1"},
		{1, "1-2"},
		{2, "2-1"},
	} {
		response := manager.Issue(spec.subID, 1, controlLeaseRequest{
			BusID:       spec.busid,
			ClientNonce: spec.subID,
		}, func() (bool, string) {
			return true, ""
		})
		require.Empty(t, response.ErrorCode, spec.busid)
	}
	require.Len(t, manager.leases, 3)

	manager.RevokeSubscriber(1)
	require.Len(t, manager.leases, 1)
	_, found := manager.leases["2-1"]
	require.True(t, found)
}
