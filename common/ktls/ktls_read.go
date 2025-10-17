// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && go1.25 && badlinkname

package ktls

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
)

func (c *Conn) Read(b []byte) (int, error) {
	if !c.kernelRx {
		return c.Conn.Read(b)
	}

	if len(b) == 0 {
		// Put this after Handshake, in case people were calling
		// Read(nil) for the side effect of the Handshake.
		return 0, nil
	}

	c.rawConn.In.Lock()
	defer c.rawConn.In.Unlock()

	for c.rawConn.Input.Len() == 0 {
		if err := c.readRecord(); err != nil {
			return 0, err
		}
		for c.rawConn.Hand.Len() > 0 {
			if err := c.handlePostHandshakeMessage(); err != nil {
				return 0, err
			}
		}
	}

	n, _ := c.rawConn.Input.Read(b)

	// If a close-notify alert is waiting, read it so that we can return (n,
	// EOF) instead of (n, nil), to signal to the HTTP response reading
	// goroutine that the connection is now closed. This eliminates a race
	// where the HTTP response reading goroutine would otherwise not observe
	// the EOF until its next read, by which time a client goroutine might
	// have already tried to reuse the HTTP connection for a new request.
	// See https://golang.org/cl/76400046 and https://golang.org/issue/3514
	if n != 0 && c.rawConn.Input.Len() == 0 && c.rawConn.RawInput.Len() > 0 &&
		c.rawConn.RawInput.Bytes()[0] == recordTypeAlert {
		if err := c.readRecord(); err != nil {
			return n, err // will be io.EOF on closeNotify
		}
	}

	return n, nil
}

func (c *Conn) readRecord() error {
	if *c.rawConn.In.Err != nil {
		return *c.rawConn.In.Err
	}

	typ, data, err := c.readRawRecord()
	if err != nil {
		return err
	}

	if len(data) > maxPlaintext {
		return c.rawConn.In.SetErrorLocked(c.sendAlert(alertRecordOverflow))
	}

	// Application Data messages are always protected.
	if c.rawConn.In.Cipher == nil && typ == recordTypeApplicationData {
		return c.rawConn.In.SetErrorLocked(c.sendAlert(alertUnexpectedMessage))
	}

	//if typ != recordTypeAlert && typ != recordTypeChangeCipherSpec && len(data) > 0 {
	// This is a state-advancing message: reset the retry count.
	// c.retryCount = 0
	//}

	// Handshake messages MUST NOT be interleaved with other record types in TLS 1.3.
	if *c.rawConn.Vers == tls.VersionTLS13 && typ != recordTypeHandshake && c.rawConn.Hand.Len() > 0 {
		return c.rawConn.In.SetErrorLocked(c.sendAlert(alertUnexpectedMessage))
	}

	switch typ {
	default:
		return c.rawConn.In.SetErrorLocked(c.sendAlert(alertUnexpectedMessage))
	case recordTypeAlert:
		//if c.quic != nil {
		//	return c.rawConn.In.setErrorLocked(c.sendAlert(alertUnexpectedMessage))
		//}
		if len(data) != 2 {
			return c.rawConn.In.SetErrorLocked(c.sendAlert(alertUnexpectedMessage))
		}
		if data[1] == alertCloseNotify {
			return c.rawConn.In.SetErrorLocked(io.EOF)
		}
		if *c.rawConn.Vers == tls.VersionTLS13 {
			// TLS 1.3 removed warning-level alerts except for alertUserCanceled
			// (RFC 8446, § 6.1). Since at least one major implementation
			// (https://bugs.openjdk.org/browse/JDK-8323517) misuses this alert,
			// many TLS stacks now ignore it outright when seen in a TLS 1.3
			// handshake (e.g. BoringSSL, NSS, Rustls).
			if data[1] == alertUserCanceled {
				// Like TLS 1.2 alertLevelWarning alerts, we drop the record and retry.
				return c.retryReadRecord( /*expectChangeCipherSpec*/ )
			}
			return c.rawConn.In.SetErrorLocked(&net.OpError{Op: "remote error", Err: tls.AlertError(data[1])})
		}
		switch data[0] {
		case alertLevelWarning:
			// Drop the record on the floor and retry.
			return c.retryReadRecord( /*expectChangeCipherSpec*/ )
		case alertLevelError:
			return c.rawConn.In.SetErrorLocked(&net.OpError{Op: "remote error", Err: tls.AlertError(data[1])})
		default:
			return c.rawConn.In.SetErrorLocked(c.sendAlert(alertUnexpectedMessage))
		}

	case recordTypeChangeCipherSpec:
		if len(data) != 1 || data[0] != 1 {
			return c.rawConn.In.SetErrorLocked(c.sendAlert(alertDecodeError))
		}
		// Handshake messages are not allowed to fragment across the CCS.
		if c.rawConn.Hand.Len() > 0 {
			return c.rawConn.In.SetErrorLocked(c.sendAlert(alertUnexpectedMessage))
		}
		// In TLS 1.3, change_cipher_spec records are ignored until the
		// Finished. See RFC 8446, Appendix D.4. Note that according to Section
		// 5, a server can send a ChangeCipherSpec before its ServerHello, when
		// c.vers is still unset. That's not useful though and suspicious if the
		// server then selects a lower protocol version, so don't allow that.
		if *c.rawConn.Vers == tls.VersionTLS13 {
			return c.retryReadRecord( /*expectChangeCipherSpec*/ )
		}
		// if !expectChangeCipherSpec {
		return c.rawConn.In.SetErrorLocked(c.sendAlert(alertUnexpectedMessage))
		//}
		//if err := c.rawConn.In.changeCipherSpec(); err != nil {
		//	return c.rawConn.In.setErrorLocked(c.sendAlert(err.(alert)))
		//}

	case recordTypeApplicationData:
		// Some OpenSSL servers send empty records in order to randomize the
		// CBC RawIV. Ignore a limited number of empty records.
		if len(data) == 0 {
			return c.retryReadRecord( /*expectChangeCipherSpec*/ )
		}
		// Note that data is owned by c.rawInput, following the Next call above,
		// to avoid copying the plaintext. This is safe because c.rawInput is
		// not read from or written to until c.input is drained.
		c.rawConn.Input.Reset(data)
	case recordTypeHandshake:
		if len(data) == 0 {
			return c.rawConn.In.SetErrorLocked(c.sendAlert(alertUnexpectedMessage))
		}
		c.rawConn.Hand.Write(data)
	}

	return nil
}

//nolint:staticcheck
func (c *Conn) readRawRecord() (typ uint8, data []byte, err error) {
	// Read from kernel.
	if c.kernelRx {
		return c.readKernelRecord()
	}

	// Read header, payload.
	if err = c.readFromUntil(c.conn, recordHeaderLen); err != nil {
		// RFC 8446, Section 6.1 suggests that EOF without an alertCloseNotify
		// is an error, but popular web sites seem to do this, so we accept it
		// if and only if at the record boundary.
		if err == io.ErrUnexpectedEOF && c.rawConn.RawInput.Len() == 0 {
			err = io.EOF
		}
		if e, ok := err.(net.Error); !ok || !e.Temporary() {
			c.rawConn.In.SetErrorLocked(err)
		}
		return
	}
	hdr := c.rawConn.RawInput.Bytes()[:recordHeaderLen]
	typ = hdr[0]

	vers := uint16(hdr[1])<<8 | uint16(hdr[2])
	expectedVers := *c.rawConn.Vers
	if expectedVers == tls.VersionTLS13 {
		// All TLS 1.3 records are expected to have 0x0303 (1.2) after
		// the initial hello (RFC 8446 Section 5.1).
		expectedVers = tls.VersionTLS12
	}
	n := int(hdr[3])<<8 | int(hdr[4])
	if /*c.haveVers && */ vers != expectedVers {
		c.sendAlert(alertProtocolVersion)
		msg := fmt.Sprintf("received record with version %x when expecting version %x", vers, expectedVers)
		err = c.rawConn.In.SetErrorLocked(c.newRecordHeaderError(nil, msg))
		return
	}
	//if !c.haveVers {
	//	// First message, be extra suspicious: this might not be a TLS
	//	// client. Bail out before reading a full 'body', if possible.
	//	// The current max version is 3.3 so if the version is >= 16.0,
	//	// it's probably not real.
	//	if (typ != recordTypeAlert && typ != recordTypeHandshake) || vers >= 0x1000 {
	//		err = c.rawConn.In.SetErrorLocked(c.newRecordHeaderError(c.conn, "first record does not look like a TLS handshake"))
	//		return
	//	}
	//}
	if *c.rawConn.Vers == tls.VersionTLS13 && n > maxCiphertextTLS13 || n > maxCiphertext {
		c.sendAlert(alertRecordOverflow)
		msg := fmt.Sprintf("oversized record received with length %d", n)
		err = c.rawConn.In.SetErrorLocked(c.newRecordHeaderError(nil, msg))
		return
	}
	if err = c.readFromUntil(c.conn, recordHeaderLen+n); err != nil {
		if e, ok := err.(net.Error); !ok || !e.Temporary() {
			c.rawConn.In.SetErrorLocked(err)
		}
		return
	}

	// Process message.
	record := c.rawConn.RawInput.Next(recordHeaderLen + n)
	data, typ, err = c.rawConn.In.Decrypt(record)
	if err != nil {
		err = c.rawConn.In.SetErrorLocked(c.sendAlert(uint8(err.(tls.AlertError))))
		return
	}
	return
}

// retryReadRecord recurs into readRecordOrCCS to drop a non-advancing record, like
// a warning alert, empty application_data, or a change_cipher_spec in TLS 1.3.
func (c *Conn) retryReadRecord( /*expectChangeCipherSpec bool*/ ) error {
	//c.retryCount++
	//if c.retryCount > maxUselessRecords {
	//	c.sendAlert(alertUnexpectedMessage)
	//	return c.in.setErrorLocked(errors.New("tls: too many ignored records"))
	//}
	return c.readRecord( /*expectChangeCipherSpec*/ )
}

// atLeastReader reads from R, stopping with EOF once at least N bytes have been
// read. It is different from an io.LimitedReader in that it doesn't cut short
// the last Read call, and in that it considers an early EOF an error.
type atLeastReader struct {
	R io.Reader
	N int64
}

func (r *atLeastReader) Read(p []byte) (int, error) {
	if r.N <= 0 {
		return 0, io.EOF
	}
	n, err := r.R.Read(p)
	r.N -= int64(n) // won't underflow unless len(p) >= n > 9223372036854775809
	if r.N > 0 && err == io.EOF {
		return n, io.ErrUnexpectedEOF
	}
	if r.N <= 0 && err == nil {
		return n, io.EOF
	}
	return n, err
}

// readFromUntil reads from r into c.rawConn.RawInput until c.rawConn.RawInput contains
// at least n bytes or else returns an error.
func (c *Conn) readFromUntil(r io.Reader, n int) error {
	if c.rawConn.RawInput.Len() >= n {
		return nil
	}
	needs := n - c.rawConn.RawInput.Len()
	// There might be extra input waiting on the wire. Make a best effort
	// attempt to fetch it so that it can be used in (*Conn).Read to
	// "predict" closeNotify alerts.
	c.rawConn.RawInput.Grow(needs + bytes.MinRead)
	_, err := c.rawConn.RawInput.ReadFrom(&atLeastReader{r, int64(needs)})
	return err
}

func (c *Conn) newRecordHeaderError(conn net.Conn, msg string) (err tls.RecordHeaderError) {
	err.Msg = msg
	err.Conn = conn
	copy(err.RecordHeader[:], c.rawConn.RawInput.Bytes())
	return err
}
