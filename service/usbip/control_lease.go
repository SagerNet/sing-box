//go:build linux || (darwin && cgo)

package usbip

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
	if request.BusID == "" {
		return controlLeaseResponse{
			BusID:        request.BusID,
			ClientNonce:  request.ClientNonce,
			ErrorCode:    leaseErrorBadRequest,
			ErrorMessage: "missing busid",
		}
	}
	s.controlAccess.Lock()
	generation := s.controlSeq
	s.controlAccess.Unlock()
	return s.leases.Issue(subID, generation, request, func() (bool, string) {
		return s.leaseAvailable(request.BusID)
	})
}

func (s *ServerService) consumeImportLease(request ImportExtRequest) bool {
	s.controlAccess.Lock()
	defer s.controlAccess.Unlock()
	return s.leases.Consume(s.controlSeq, request)
}

func (s *ServerService) currentControlSequence() uint64 {
	s.controlAccess.Lock()
	defer s.controlAccess.Unlock()
	return s.controlSeq
}
