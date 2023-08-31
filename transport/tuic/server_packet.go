//go:build with_quic

package tuic

import (
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

func (s *serverSession) loopMessages() {
	select {
	case <-s.connDone:
		return
	case <-s.authDone:
	}
	for {
		message, err := s.quicConn.ReceiveMessage(s.ctx)
		if err != nil {
			s.closeWithError(E.Cause(err, "receive message"))
			return
		}
		hErr := s.handleMessage(message)
		if hErr != nil {
			s.closeWithError(E.Cause(hErr, "handle message"))
			return
		}
	}
}

func (s *serverSession) handleMessage(data []byte) error {
	if len(data) < 2 {
		return E.New("invalid message")
	}
	if data[0] != Version {
		return E.New("unknown version ", data[0])
	}
	switch data[1] {
	case CommandPacket:
		message := udpMessagePool.Get().(*udpMessage)
		err := decodeUDPMessage(message, data[2:])
		if err != nil {
			message.release()
			return E.Cause(err, "decode UDP message")
		}
		s.handleUDPMessage(message, false)
		return nil
	case CommandHeartbeat:
		return nil
	default:
		return E.New("unknown command ", data[0])
	}
}

func (s *serverSession) handleUDPMessage(message *udpMessage, udpStream bool) {
	s.udpAccess.RLock()
	udpConn, loaded := s.udpConnMap[message.sessionID]
	s.udpAccess.RUnlock()
	if !loaded || common.Done(udpConn.ctx) {
		udpConn = newUDPPacketConn(s.ctx, s.quicConn, udpStream, true, func() {
			s.udpAccess.Lock()
			delete(s.udpConnMap, message.sessionID)
			s.udpAccess.Unlock()
		})
		udpConn.sessionID = message.sessionID
		s.udpAccess.Lock()
		s.udpConnMap[message.sessionID] = udpConn
		s.udpAccess.Unlock()
		go s.handler.NewPacketConnection(udpConn.ctx, udpConn, M.Metadata{
			Source:      s.source,
			Destination: message.destination,
		})
	}
	udpConn.inputPacket(message)
}
