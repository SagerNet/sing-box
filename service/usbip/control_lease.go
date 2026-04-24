//go:build linux || (darwin && cgo)

package usbip

import (
	"time"
)

func (s *ServerService) handleControlLeaseRequest(sub *serverControlConn, payload []byte) {
	var request controlLeaseRequest
	err := unmarshalControlPayload(payload, &request)
	if err != nil {
		s.enqueueControlPayload(sub, controlFrame{
			Type:    controlFrameLeaseResponse,
			Version: controlProtocolVersion,
		}, controlLeaseResponse{
			ErrorCode:    leaseErrorBadRequest,
			ErrorMessage: err.Error(),
		}, controlFrame{Type: controlFrameChanged, Version: controlProtocolVersion, Sequence: s.currentControlSequence()})
		return
	}
	response := s.createControlLeaseResponse(sub.id, request)
	s.enqueueControlPayload(sub, controlFrame{
		Type:    controlFrameLeaseResponse,
		Version: controlProtocolVersion,
	}, response, controlFrame{Type: controlFrameChanged, Version: controlProtocolVersion, Sequence: s.currentControlSequence()})
}

func (s *ServerService) createControlLeaseResponse(subID uint64, request controlLeaseRequest) controlLeaseResponse {
	response := controlLeaseResponse{
		BusID:       request.BusID,
		ClientNonce: request.ClientNonce,
	}
	if request.BusID == "" {
		response.ErrorCode = leaseErrorBadRequest
		response.ErrorMessage = "missing busid"
		return response
	}
	ok, reason := s.leaseAvailable(request.BusID)
	if !ok {
		response.ErrorCode = leaseErrorUnavailable
		response.ErrorMessage = reason
		return response
	}
	now := time.Now()
	expires := now.Add(importLeaseTTL)
	s.controlAccess.Lock()
	defer s.controlAccess.Unlock()
	s.cleanupExpiredImportLeasesLocked(now)
	_, exists := s.leasesByBusID[request.BusID]
	if exists {
		response.ErrorCode = leaseErrorBusy
		response.ErrorMessage = "lease already active"
		return response
	}
	s.leaseNextID++
	lease := serverImportLease{
		ID:           s.leaseNextID,
		SubscriberID: subID,
		BusID:        request.BusID,
		ClientNonce:  request.ClientNonce,
		Generation:   s.controlSeq,
		Expires:      expires,
	}
	s.leasesByBusID[lease.BusID] = lease
	response.LeaseID = lease.ID
	response.Generation = lease.Generation
	response.TTLMillis = int64(importLeaseTTL / time.Millisecond)
	return response
}

func (s *ServerService) consumeImportLease(request ImportExtRequest) bool {
	now := time.Now()
	s.controlAccess.Lock()
	defer s.controlAccess.Unlock()
	s.cleanupExpiredImportLeasesLocked(now)
	lease, ok := s.leasesByBusID[request.BusID]
	if !ok {
		return false
	}
	if lease.ID != request.LeaseID || lease.ClientNonce != request.ClientNonce {
		return false
	}
	delete(s.leasesByBusID, request.BusID)
	return now.Before(lease.Expires) && lease.Generation == s.controlSeq
}

func (s *ServerService) cleanupExpiredImportLeasesLocked(now time.Time) {
	for busid, lease := range s.leasesByBusID {
		if now.Before(lease.Expires) {
			continue
		}
		delete(s.leasesByBusID, busid)
	}
}

func (s *ServerService) deleteImportLeasesForSubscriberLocked(subID uint64) {
	for busid, lease := range s.leasesByBusID {
		if lease.SubscriberID != subID {
			continue
		}
		delete(s.leasesByBusID, busid)
	}
}

func (s *ServerService) currentControlSequence() uint64 {
	s.controlAccess.Lock()
	defer s.controlAccess.Unlock()
	return s.controlSeq
}
