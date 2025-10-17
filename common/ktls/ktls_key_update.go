// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && go1.25 && badlinkname

package ktls

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"os"
)

// handlePostHandshakeMessage processes a handshake message arrived after the
// handshake is complete. Up to TLS 1.2, it indicates the start of a renegotiation.
func (c *Conn) handlePostHandshakeMessage() error {
	if *c.rawConn.Vers != tls.VersionTLS13 {
		return errors.New("ktls: kernel does not support TLS 1.2 renegotiation")
	}

	msg, err := c.readHandshake(nil)
	if err != nil {
		return err
	}
	//c.retryCount++
	//if c.retryCount > maxUselessRecords {
	//	c.sendAlert(alertUnexpectedMessage)
	//	return c.in.setErrorLocked(errors.New("tls: too many non-advancing records"))
	//}

	switch msg := msg.(type) {
	case *newSessionTicketMsgTLS13:
		// return errors.New("ktls: received new session ticket")
		return nil
	case *keyUpdateMsg:
		return c.handleKeyUpdate(msg)
	}
	// The QUIC layer is supposed to treat an unexpected post-handshake CertificateRequest
	// as a QUIC-level PROTOCOL_VIOLATION error (RFC 9001, Section 4.4). Returning an
	// unexpected_message alert here doesn't provide it with enough information to distinguish
	// this condition from other unexpected messages. This is probably fine.
	c.sendAlert(alertUnexpectedMessage)
	return fmt.Errorf("tls: received unexpected handshake message of type %T", msg)
}

func (c *Conn) handleKeyUpdate(keyUpdate *keyUpdateMsg) error {
	//if c.quic != nil {
	//	c.sendAlert(alertUnexpectedMessage)
	//	return c.in.setErrorLocked(errors.New("tls: received unexpected key update message"))
	//}

	cipherSuite := cipherSuiteTLS13ByID(*c.rawConn.CipherSuite)
	if cipherSuite == nil {
		return c.rawConn.In.SetErrorLocked(c.sendAlert(alertInternalError))
	}

	newSecret := nextTrafficSecret(cipherSuite, *c.rawConn.In.TrafficSecret)
	c.rawConn.In.SetTrafficSecret(cipherSuite, 0 /*tls.QUICEncryptionLevelInitial*/, newSecret)

	err := c.resetupRX()
	if err != nil {
		c.sendAlert(alertInternalError)
		return c.rawConn.In.SetErrorLocked(fmt.Errorf("ktls: resetupRX failed: %w", err))
	}

	if keyUpdate.updateRequested {
		c.rawConn.Out.Lock()
		defer c.rawConn.Out.Unlock()

		resetup, err := c.resetupTX()
		if err != nil {
			c.sendAlertLocked(alertInternalError)
			return c.rawConn.Out.SetErrorLocked(fmt.Errorf("ktls: resetupTX failed: %w", err))
		}

		msg := &keyUpdateMsg{}
		msgBytes, err := msg.marshal()
		if err != nil {
			return err
		}
		_, err = c.writeRecordLocked(recordTypeHandshake, msgBytes)
		if err != nil {
			// Surface the error at the next write.
			c.rawConn.Out.SetErrorLocked(err)
			return nil
		}

		newSecret := nextTrafficSecret(cipherSuite, *c.rawConn.Out.TrafficSecret)
		c.rawConn.Out.SetTrafficSecret(cipherSuite, 0 /*QUICEncryptionLevelInitial*/, newSecret)

		err = resetup()
		if err != nil {
			return c.rawConn.Out.SetErrorLocked(fmt.Errorf("ktls: resetupTX failed: %w", err))
		}
	}

	return nil
}

func (c *Conn) readHandshakeBytes(n int) error {
	//if c.quic != nil {
	//	return c.quicReadHandshakeBytes(n)
	//}
	for c.rawConn.Hand.Len() < n {
		if err := c.readRecord(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conn) readHandshake(transcript io.Writer) (any, error) {
	if err := c.readHandshakeBytes(4); err != nil {
		return nil, err
	}
	data := c.rawConn.Hand.Bytes()

	maxHandshakeSize := maxHandshake
	// hasVers indicates we're past the first message, forcing someone trying to
	// make us just allocate a large buffer to at least do the initial part of
	// the handshake first.
	//if c.haveVers && data[0] == typeCertificate {
	// Since certificate messages are likely to be the only messages that
	// can be larger than maxHandshake, we use a special limit for just
	// those messages.
	//maxHandshakeSize = maxHandshakeCertificateMsg
	//}

	n := int(data[1])<<16 | int(data[2])<<8 | int(data[3])
	if n > maxHandshakeSize {
		c.sendAlertLocked(alertInternalError)
		return nil, c.rawConn.In.SetErrorLocked(fmt.Errorf("tls: handshake message of length %d bytes exceeds maximum of %d bytes", n, maxHandshakeSize))
	}
	if err := c.readHandshakeBytes(4 + n); err != nil {
		return nil, err
	}
	data = c.rawConn.Hand.Next(4 + n)
	return c.unmarshalHandshakeMessage(data, transcript)
}

func (c *Conn) unmarshalHandshakeMessage(data []byte, transcript io.Writer) (any, error) {
	var m handshakeMessage
	switch data[0] {
	case typeNewSessionTicket:
		if *c.rawConn.Vers == tls.VersionTLS13 {
			m = new(newSessionTicketMsgTLS13)
		} else {
			return nil, os.ErrInvalid
		}
	case typeKeyUpdate:
		m = new(keyUpdateMsg)
	default:
		return nil, c.rawConn.In.SetErrorLocked(c.sendAlert(alertUnexpectedMessage))
	}

	// The handshake message unmarshalers
	// expect to be able to keep references to data,
	// so pass in a fresh copy that won't be overwritten.
	data = append([]byte(nil), data...)

	if !m.unmarshal(data) {
		return nil, c.rawConn.In.SetErrorLocked(c.sendAlert(alertDecodeError))
	}

	if transcript != nil {
		transcript.Write(data)
	}

	return m, nil
}
