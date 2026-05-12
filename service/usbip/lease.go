package usbip

import (
	"sync"
	"time"
)

// LeaseManager owns the server's import-lease state machine.
//
// Lock ordering rules for callers integrating with ServerService:
//
//	ServerService.controlAccess -> LeaseManager.access -> ServerService.access
//
// Concretely:
//   - The `available` callback supplied to Issue may acquire
//     ServerService.access; it MUST NOT acquire ServerService.controlAccess.
//   - Callers may hold ServerService.controlAccess across Consume or
//     RevokeSubscriber (both are pure map operations, fast under the lock).
//   - Callers MUST NOT hold ServerService.controlAccess across Issue,
//     because Issue runs `available` which can syscall on Linux
//     (readUsbipStatus) and would otherwise stall control broadcasts.
type LeaseManager struct {
	access sync.Mutex
	leases map[string]serverImportLease
	nextID uint64
	ttl    time.Duration
	now    func() time.Time
}

func NewLeaseManager(ttl time.Duration, now func() time.Time) *LeaseManager {
	if now == nil {
		now = time.Now
	}
	return &LeaseManager{
		leases: make(map[string]serverImportLease),
		ttl:    ttl,
		now:    now,
	}
}

// Issue atomically checks availability and inserts a fresh lease. The
// `available` closure is invoked under the manager's lock, so the check
// and the insert are TOCTOU-free against any state that `available`
// observes. The closure must respect the lock ordering documented above.
func (m *LeaseManager) Issue(
	subID, generation uint64,
	request controlLeaseRequest,
	available func() (bool, string),
) controlLeaseResponse {
	response := controlLeaseResponse{
		BusID:       request.BusID,
		ClientNonce: request.ClientNonce,
	}
	m.access.Lock()
	defer m.access.Unlock()
	now := m.now()
	m.cleanupExpiredLocked(now)
	ok, reason := available()
	if !ok {
		response.ErrorCode = leaseErrorUnavailable
		response.ErrorMessage = reason
		return response
	}
	_, exists := m.leases[request.BusID]
	if exists {
		response.ErrorCode = leaseErrorBusy
		response.ErrorMessage = "lease already active"
		return response
	}
	m.nextID++
	lease := serverImportLease{
		ID:           m.nextID,
		SubscriberID: subID,
		BusID:        request.BusID,
		ClientNonce:  request.ClientNonce,
		Generation:   generation,
		Expires:      now.Add(m.ttl),
	}
	m.leases[request.BusID] = lease
	response.LeaseID = lease.ID
	response.Generation = lease.Generation
	response.TTLMillis = int64(m.ttl / time.Millisecond)
	return response
}

// Consume validates and removes the lease referenced by request. It
// returns true only if the lease matches ID + nonce, is not expired,
// and lease.Generation matches currentGeneration. The lease entry is
// removed regardless of outcome (consume-on-read semantics).
func (m *LeaseManager) Consume(currentGeneration uint64, request ImportExtRequest) bool {
	m.access.Lock()
	defer m.access.Unlock()
	now := m.now()
	m.cleanupExpiredLocked(now)
	lease, found := m.leases[request.BusID]
	if !found {
		return false
	}
	if lease.ID != request.LeaseID || lease.ClientNonce != request.ClientNonce {
		return false
	}
	delete(m.leases, request.BusID)
	return now.Before(lease.Expires) && lease.Generation == currentGeneration
}

// RevokeSubscriber removes every lease previously issued to subID.
// Called when a control subscriber disconnects.
func (m *LeaseManager) RevokeSubscriber(subID uint64) {
	m.access.Lock()
	defer m.access.Unlock()
	for busid, lease := range m.leases {
		if lease.SubscriberID == subID {
			delete(m.leases, busid)
		}
	}
}

func (m *LeaseManager) cleanupExpiredLocked(now time.Time) {
	for busid, lease := range m.leases {
		if !now.Before(lease.Expires) {
			delete(m.leases, busid)
		}
	}
}
