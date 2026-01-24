// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && go1.25 && badlinkname

package ktls

import (
	"crypto/tls"
	"net"
)

const (
	// alert level
	alertLevelWarning = 1
	alertLevelError   = 2
)

const (
	alertCloseNotify                  = 0
	alertUnexpectedMessage            = 10
	alertBadRecordMAC                 = 20
	alertDecryptionFailed             = 21
	alertRecordOverflow               = 22
	alertDecompressionFailure         = 30
	alertHandshakeFailure             = 40
	alertBadCertificate               = 42
	alertUnsupportedCertificate       = 43
	alertCertificateRevoked           = 44
	alertCertificateExpired           = 45
	alertCertificateUnknown           = 46
	alertIllegalParameter             = 47
	alertUnknownCA                    = 48
	alertAccessDenied                 = 49
	alertDecodeError                  = 50
	alertDecryptError                 = 51
	alertExportRestriction            = 60
	alertProtocolVersion              = 70
	alertInsufficientSecurity         = 71
	alertInternalError                = 80
	alertInappropriateFallback        = 86
	alertUserCanceled                 = 90
	alertNoRenegotiation              = 100
	alertMissingExtension             = 109
	alertUnsupportedExtension         = 110
	alertCertificateUnobtainable      = 111
	alertUnrecognizedName             = 112
	alertBadCertificateStatusResponse = 113
	alertBadCertificateHashValue      = 114
	alertUnknownPSKIdentity           = 115
	alertCertificateRequired          = 116
	alertNoApplicationProtocol        = 120
	alertECHRequired                  = 121
)

func (c *Conn) sendAlertLocked(err uint8) error {
	switch err {
	case alertNoRenegotiation, alertCloseNotify:
		c.rawConn.Tmp[0] = alertLevelWarning
	default:
		c.rawConn.Tmp[0] = alertLevelError
	}
	c.rawConn.Tmp[1] = byte(err)

	_, writeErr := c.writeRecordLocked(recordTypeAlert, c.rawConn.Tmp[0:2])
	if err == alertCloseNotify {
		// closeNotify is a special case in that it isn't an error.
		return writeErr
	}

	return c.rawConn.Out.SetErrorLocked(&net.OpError{Op: "local error", Err: tls.AlertError(err)})
}

// sendAlert sends a TLS alert message.
func (c *Conn) sendAlert(err uint8) error {
	c.rawConn.Out.Lock()
	defer c.rawConn.Out.Unlock()
	return c.sendAlertLocked(err)
}
