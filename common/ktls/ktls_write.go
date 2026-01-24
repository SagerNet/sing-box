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

func (c *Conn) writeRecordLocked(typ uint16, data []byte) (n int, err error) {
	if !c.kernelTx {
		return c.rawConn.WriteRecordLocked(typ, data)
	}
	/*for len(data) > 0 {
		m := len(data)
		if maxPayload := c.maxPayloadSizeForWrite(typ); m > maxPayload {
			m = maxPayload
		}
		_, err = c.writeKernelRecord(typ, data[:m])
		if err != nil {
			return
		}
		n += m
		data = data[m:]
	}*/
	return c.writeKernelRecord(typ, data)
}

const (
	// tcpMSSEstimate is a conservative estimate of the TCP maximum segment
	// size (MSS). A constant is used, rather than querying the kernel for
	// the actual MSS, to avoid complexity. The value here is the IPv6
	// minimum MTU (1280 bytes) minus the overhead of an IPv6 header (40
	// bytes) and a TCP header with timestamps (32 bytes).
	tcpMSSEstimate = 1208

	// recordSizeBoostThreshold is the number of bytes of application data
	// sent after which the TLS record size will be increased to the
	// maximum.
	recordSizeBoostThreshold = 128 * 1024
)

func (c *Conn) maxPayloadSizeForWrite(typ uint16) int {
	if /*c.config.DynamicRecordSizingDisabled ||*/ typ != recordTypeApplicationData {
		return maxPlaintext
	}

	if *c.rawConn.PacketsSent >= recordSizeBoostThreshold {
		return maxPlaintext
	}

	// Subtract TLS overheads to get the maximum payload size.
	payloadBytes := tcpMSSEstimate - recordHeaderLen - c.rawConn.Out.ExplicitNonceLen()
	if rawCipher := *c.rawConn.Out.Cipher; rawCipher != nil {
		switch ciph := rawCipher.(type) {
		case cipher.Stream:
			payloadBytes -= (*c.rawConn.Out.Mac).Size()
		case cipher.AEAD:
			payloadBytes -= ciph.Overhead()
		/*case cbcMode:
		blockSize := ciph.BlockSize()
		// The payload must fit in a multiple of blockSize, with
		// room for at least one padding byte.
		payloadBytes = (payloadBytes & ^(blockSize - 1)) - 1
		// The RawMac is appended before padding so affects the
		// payload size directly.
		payloadBytes -= c.out.mac.Size()*/
		default:
			panic("unknown cipher type")
		}
	}
	if *c.rawConn.Vers == tls.VersionTLS13 {
		payloadBytes-- // encrypted ContentType
	}

	// Allow packet growth in arithmetic progression up to max.
	pkt := *c.rawConn.PacketsSent
	*c.rawConn.PacketsSent++
	if pkt > 1000 {
		return maxPlaintext // avoid overflow in multiply below
	}

	n := payloadBytes * int(pkt+1)
	if n > maxPlaintext {
		n = maxPlaintext
	}
	return n
}
