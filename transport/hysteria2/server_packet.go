package hysteria2

import (
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

func (s *serverSession) loopMessages() {
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
	message := udpMessagePool.Get().(*udpMessage)
	err := decodeUDPMessage(message, data)
	if err != nil {
		message.release()
		return E.Cause(err, "decode UDP message")
	}
	s.handleUDPMessage(message)
	return nil
}

func (s *serverSession) handleUDPMessage(message *udpMessage) {
	s.udpAccess.RLock()
	udpConn, loaded := s.udpConnMap[message.sessionID]
	s.udpAccess.RUnlock()
	if !loaded || common.Done(udpConn.ctx) {
		udpConn = newUDPPacketConn(s.ctx, s.quicConn, func() {
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
			Destination: M.ParseSocksaddr(message.destination),
		})
	}
	udpConn.inputPacket(message)
}
