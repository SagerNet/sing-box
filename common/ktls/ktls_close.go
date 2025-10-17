// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && go1.25 && badlinkname

package ktls

import (
	"fmt"
	"net"
	"time"
)

func (c *Conn) Close() error {
	if !c.kernelTx {
		return c.Conn.Close()
	}

	// Interlock with Conn.Write above.
	var x int32
	for {
		x = c.rawConn.ActiveCall.Load()
		if x&1 != 0 {
			return net.ErrClosed
		}
		if c.rawConn.ActiveCall.CompareAndSwap(x, x|1) {
			break
		}
	}
	if x != 0 {
		// io.Writer and io.Closer should not be used concurrently.
		// If Close is called while a Write is currently in-flight,
		// interpret that as a sign that this Close is really just
		// being used to break the Write and/or clean up resources and
		// avoid sending the alertCloseNotify, which may block
		// waiting on handshakeMutex or the c.out mutex.
		return c.conn.Close()
	}

	var alertErr error
	if c.rawConn.IsHandshakeComplete.Load() {
		if err := c.closeNotify(); err != nil {
			alertErr = fmt.Errorf("tls: failed to send closeNotify alert (but connection was closed anyway): %w", err)
		}
	}

	if err := c.conn.Close(); err != nil {
		return err
	}
	return alertErr
}

func (c *Conn) closeNotify() error {
	c.rawConn.Out.Lock()
	defer c.rawConn.Out.Unlock()

	if !*c.rawConn.CloseNotifySent {
		// Set a Write Deadline to prevent possibly blocking forever.
		c.SetWriteDeadline(time.Now().Add(time.Second * 5))
		*c.rawConn.CloseNotifyErr = c.sendAlertLocked(alertCloseNotify)
		*c.rawConn.CloseNotifySent = true
		// Any subsequent writes will fail.
		c.SetWriteDeadline(time.Now())
	}
	return *c.rawConn.CloseNotifyErr
}
