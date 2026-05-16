//go:build linux || (darwin && cgo)

package usbip

import (
	"bytes"
	"io"
	"net"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
)

func (s *ServerService) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
			}
			if E.IsClosed(err) {
				return
			}
			//nolint:staticcheck // net.Error.Temporary predates net.ErrClosed; replacement needs a separate audit.
			if netError, isNetError := err.(net.Error); isNetError && netError.Temporary() {
				s.logger.Error("accept: ", err)
				if !sleepCtx(s.ctx, 200*time.Millisecond) {
					return
				}
				continue
			}
			s.logger.Error("accept: ", err)
			return
		}
		go s.dispatchConn(conn)
	}
}

func (s *ServerService) dispatchConn(conn net.Conn) {
	var prefix [controlPrefaceSize]byte
	_, err := io.ReadFull(conn, prefix[:])
	if err != nil {
		s.logger.Debug("read connection preface: ", err)
		_ = conn.Close()
		return
	}
	if bytes.Equal(prefix[:], controlPreface[:]) {
		s.handleControlConn(conn)
		return
	}
	s.handleStandardConn(conn, ParseOpHeader(prefix[:]))
}

func (s *ServerService) readControlConn(sub *exportSubscriber, done chan<- struct{}) {
	defer close(done)
	var reader controlReader
	for {
		message, err := reader.read(sub.conn)
		if err != nil {
			return
		}
		frame := message.Frame
		switch frame.Type {
		case controlFramePing:
			s.ledger.enqueueFrame(sub, controlFrame{
				Type:    controlFramePong,
				Version: controlProtocolVersion,
			})
		case controlFrameLeaseRequest:
			if supportsControlExtensions(sub.capabilities) {
				s.ledger.HandleControlLeaseRequest(s.ctx, sub, message.Payload)
				continue
			}
			return
		default:
			return
		}
	}
}
