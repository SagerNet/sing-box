//go:build linux || (darwin && cgo)

package usbip

import (
	"net"
)

type serverControlConn struct {
	id           uint64
	capabilities uint32
	conn         net.Conn
	send         chan controlMessage
}

func (s *ServerService) registerControlConn(conn net.Conn, capabilities uint32) (*serverControlConn, uint64) {
	s.controlAccess.Lock()
	defer s.controlAccess.Unlock()
	s.controlNextID++
	sequence := s.controlSeq
	sub := &serverControlConn{
		id:           s.controlNextID,
		capabilities: capabilities,
		conn:         conn,
		send:         make(chan controlMessage, 16),
	}
	if supportsControlExtensions(capabilities) {
		s.enqueueControlSnapshot(sub, sequence)
	}
	s.controlSubs[sub.id] = sub
	return sub, sequence
}

func (s *ServerService) unregisterControlConn(id uint64) {
	s.controlAccess.Lock()
	defer s.controlAccess.Unlock()
	delete(s.controlSubs, id)
	s.deleteImportLeasesForSubscriberLocked(id)
}

func (s *ServerService) closeControlSubscribers() {
	s.controlAccess.Lock()
	subs := make([]*serverControlConn, 0, len(s.controlSubs))
	for _, sub := range s.controlSubs {
		subs = append(subs, sub)
	}
	s.controlSubs = make(map[uint64]*serverControlConn)
	s.controlAccess.Unlock()
	for _, sub := range subs {
		_ = sub.conn.Close()
	}
}

func (s *ServerService) broadcastControlState(nextState map[string]DeviceInfoV2, force bool) bool {
	s.controlAccess.Lock()
	nextSequence := s.controlSeq + 1
	delta := buildControlDeviceDelta(nextSequence, s.controlState, nextState)
	if !force && controlDeviceDeltaEmpty(delta) {
		s.controlState = nextState
		s.controlAccess.Unlock()
		return false
	}
	s.controlSeq = nextSequence
	sequence := s.controlSeq
	s.controlState = nextState
	subs := make([]*serverControlConn, 0, len(s.controlSubs))
	for _, sub := range s.controlSubs {
		subs = append(subs, sub)
	}
	s.controlAccess.Unlock()

	frame := controlFrame{
		Type:     controlFrameChanged,
		Version:  controlProtocolVersion,
		Sequence: sequence,
	}
	for _, sub := range subs {
		if supportsControlExtensions(sub.capabilities) {
			s.enqueueControlPayload(sub, controlFrame{
				Type:     controlFrameDeviceDelta,
				Version:  controlProtocolVersion,
				Sequence: sequence,
			}, delta, frame)
			continue
		}
		s.enqueueControlFrame(sub, frame)
	}
	return true
}

func (s *ServerService) enqueueControlFrame(sub *serverControlConn, frame controlFrame) {
	s.enqueueControlMessage(sub, controlMessage{Frame: frame})
}

func (s *ServerService) enqueueControlPayload(sub *serverControlConn, frame controlFrame, payload any, fallback controlFrame) {
	rawPayload, err := marshalControlPayload(payload)
	if err != nil || len(rawPayload) > maxControlPayloadLength {
		s.enqueueControlFrame(sub, fallback)
		return
	}
	s.enqueueControlMessage(sub, controlMessage{Frame: frame, Payload: rawPayload})
}

func (s *ServerService) enqueueControlSnapshot(sub *serverControlConn, sequence uint64) {
	devices := s.buildDeviceStateV2()
	s.enqueueControlPayload(sub, controlFrame{
		Type:     controlFrameDeviceSnapshot,
		Version:  controlProtocolVersion,
		Sequence: sequence,
	}, controlDeviceSnapshot{Sequence: sequence, Devices: devices}, controlFrame{
		Type:     controlFrameChanged,
		Version:  controlProtocolVersion,
		Sequence: sequence,
	})
}

func (s *ServerService) enqueueControlMessage(sub *serverControlConn, message controlMessage) {
	select {
	case sub.send <- message:
	default:
		s.logger.Debug("control subscriber ", sub.id, " lagged behind")
		_ = sub.conn.Close()
	}
}

func (s *ServerService) refreshControlState() {
	s.setControlState(deviceInfoV2Map(s.buildDeviceStateV2()))
}

func (s *ServerService) setControlState(nextState map[string]DeviceInfoV2) {
	s.controlAccess.Lock()
	s.controlState = nextState
	s.controlAccess.Unlock()
}
