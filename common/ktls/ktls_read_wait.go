// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && go1.25 && badlinkname

package ktls

import (
	"github.com/sagernet/sing/common/buf"
	N "github.com/sagernet/sing/common/network"
)

func (c *Conn) InitializeReadWaiter(options N.ReadWaitOptions) (needCopy bool) {
	c.readWaitOptions = options
	return false
}

func (c *Conn) WaitReadBuffer() (buffer *buf.Buffer, err error) {
	c.rawConn.In.Lock()
	defer c.rawConn.In.Unlock()
	for c.rawConn.Input.Len() == 0 {
		err = c.readRecord()
		if err != nil {
			return
		}
	}
	buffer = c.readWaitOptions.NewBuffer()
	n, err := c.rawConn.Input.Read(buffer.FreeBytes())
	if err != nil {
		buffer.Release()
		return
	}
	buffer.Truncate(n)
	if n != 0 && c.rawConn.Input.Len() == 0 && c.rawConn.Input.Len() > 0 &&
		c.rawConn.RawInput.Bytes()[0] == recordTypeAlert {
		_ = c.rawConn.ReadRecord()
	}
	c.readWaitOptions.PostReturn(buffer)
	return
}
