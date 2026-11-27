// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && go1.25 && badlinkname

package ktls

import (
	"crypto/cipher"
	"crypto/tls"
	"errors"
	"net"
)

func (c *Conn) Write(b []byte) (int, error) {
	if !c.kernelTx {
		return c.Conn.Write(b)
	}
	// interlock with Close below
	for {
		x := c.rawConn.ActiveCall.Load()
		if x&1 != 0 {
			return 0, net.ErrClosed
		}
		if c.rawConn.ActiveCall.CompareAndSwap(x, x+2) {
			break
		}
	}
	defer c.rawConn.ActiveCall.Add(-2)

	//if err := c.Conn.HandshakeContext(context.Background()); err != nil {
	//	return 0, err
	//}

	c.rawConn.Out.Lock()
	defer c.rawConn.Out.Unlock()

	if err := *c.rawConn.Out.Err; err != nil {
		return 0, err
	}

	if !c.rawConn.IsHandshakeComplete.Load() {
		return 0, tls.AlertError(alertInternalError)
	}

	if *c.rawConn.CloseNotifySent {
		// return 0, errShutdown
		return 0, errors.New("tls: protocol is shutdown")
	}

	// TLS 1.0 is susceptible to a chosen-plaintext
	// attack when using block mode ciphers due to predictable IVs.
	// This can be prevented by splitting each Application Data
	// record into two records, effectively randomizing the RawIV.
	//
	// https://www.openssl.org/~bodo/tls-cbc.txt
	// https://bugzilla.mozilla.org/show_bug.cgi?id=665814
	// https://www.imperialviolet.org/2012/01/15/beastfollowup.html

	var m int
	if len(b) > 1 && *c.rawConn.Vers == tls.VersionTLS10 {
		if _, ok := (*c.rawConn.Out.Cipher).(cipher.BlockMode); ok {
			n, err := c.writeRecordLocked(recordTypeApplicationData, b[:1])
			if err != nil {
				return n, c.rawConn.Out.SetErrorLocked(err)
			}
			m, b = 1, b[1:]
		}
	}

	n, err := c.writeRecordLocked(recordTypeApplicationData, b)
	return n + m, c.rawConn.Out.SetErrorLocked(err)
}

func (c *Conn) writeRecordLocked(typ uint8, data []byte) (n int, err error) {
	if !c.kernelTx {
		return c.rawConn.WriteRecordLocked(typ, data)
	}
	return c.writeKernelRecord(typ, data)
}
