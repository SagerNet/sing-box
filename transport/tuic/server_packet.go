package tuic

import (
	"bytes"
	"encoding/binary"

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
		message, err := s.quicConn.ReceiveMessage()
		if err != nil {
			s.closeWithError(E.Cause(err, "receive message"))
			return
		}
		go func() {
			hErr := s.handleMessage(message)
			if hErr != nil {
				s.closeWithError(E.Cause(hErr, "handle message"))
			}
		}()
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
		err := decodeUDPMessage(message, bytes.NewReader(data[2:]))
		if err != nil {
			message.release()
			return E.Cause(err, "decode UDP message")
		}
		s.handleUDPMessage(message, false)
		return nil
	case CommandDissociate:
		if len(data) != 4 {
			return E.New("invalid dissociate message")
		}
		sessionID := binary.BigEndian.Uint16(data[2:])
		s.udpAccess.RLock()
		udpConn, loaded := s.udpConnMap[sessionID]
		s.udpAccess.RUnlock()
		if loaded {
			udpConn.closeWithError(E.New("remote closed"))
			s.udpAccess.Lock()
			delete(s.udpConnMap, sessionID)
			s.udpAccess.Unlock()
		}
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
		ctx, cancel := common.ContextWithCancelCause(s.ctx)
		udpConn = &udpPacketConn{
			ctx:       ctx,
			cancel:    cancel,
			connId:    message.sessionID,
			quicConn:  s.quicConn,
			data:      make(chan *udpMessage, 64),
			udpStream: udpStream,
			isServer:  true,
		}
		s.udpAccess.Lock()
		s.udpConnMap[message.sessionID] = udpConn
		s.udpAccess.Unlock()
		go s.handler.NewPacketConnection(ctx, udpConn, M.Metadata{
			Source:      s.source,
			Destination: message.destination,
		})
	}
	if message.fragmentTotal <= 1 {
		udpConn.data <- message
	} else {
		newMessage := s.defragger.feed(message)
		if newMessage != nil {
			udpConn.data <- newMessage
		}
	}
}
